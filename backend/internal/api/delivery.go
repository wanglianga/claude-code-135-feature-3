package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// 成熟度分批发货
// 状态机：draft（农场/平台编制，逐轮写明预计重量与口感差异）
//   → submitted（采摘计划同步包装组）
//   → pack_confirmed（包装组确认采摘计划）
//   → adopter_confirmed（认养人确认 → 物流计划 shipments / 采摘任务 harvest_tasks / 权益台账 rights_records 同步生成）
//   → archived（全部轮次送达、权益余额为零后权益归档）。

type roundIn struct {
	Seq          int64   `json:"seq"`
	EstWeightKg  float64 `json:"est_weight_kg"`
	TasteDiff    string  `json:"taste_diff"`
	MaturityNote string  `json:"maturity_note"`
	PlannedDate  string  `json:"planned_date"`
	PackSpec     string  `json:"pack_spec"`
	ColdRoute    string  `json:"cold_route"`
}

// handleProjectDeliveryPlans GET 列表 / POST 编制成熟度分批发货方案
func (s *Server) handleProjectDeliveryPlans(w http.ResponseWriter, r *http.Request, projectID int64, batchNo string, ownFarm bool) {
	u := s.currentUser(r)
	switch r.Method {
	case http.MethodGet:
		ids, err := s.queryRows(r.Context(),
			`SELECT id FROM delivery_plans WHERE project_id=$1 ORDER BY id DESC`, projectID)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		out := make([]map[string]interface{}, 0, len(ids))
		for _, row := range ids {
			pid, _ := asID(row["id"])
			detail, e := s.planDetail(r.Context(), pid)
			if e != nil {
				continue
			}
			// 认养人/外部角色只能看到本人订单的方案；农场/客服/管理员看全部
			owner := asID64(detail["adoption_user_id"])
			if !(ownFarm || u.Role == "service" || u.Role == "admin" || owner == u.ID) {
				continue
			}
			out = append(out, detail)
		}
		writeJSON(w, 200, out)

	case http.MethodPost:
		if !ownFarm && u.Role != "admin" {
			writeErr(w, 403, "只有农场方可以按成熟度编制分批发货方案")
			return
		}
		var in struct {
			AdoptionID int64     `json:"adoption_id"`
			Reason     string    `json:"reason"`
			Rounds     []roundIn `json:"rounds"`
		}
		if err := decode(r, &in); err != nil {
			writeErr(w, 400, "请求格式错误")
			return
		}
		if in.AdoptionID == 0 {
			writeErr(w, 400, "adoption_id 必填（方案按认养订单编制）")
			return
		}
		if len(in.Rounds) < 2 {
			writeErr(w, 400, "成熟度分批至少需要 2 轮发货，并逐轮写明预计重量与口感差异")
			return
		}
		ctx := r.Context()
		var ownerID, orderProject, plotID int64
		var shares int64
		var guarantee float64
		err := s.Pool.QueryRow(ctx, `
			SELECT a.user_id, a.project_id, p.plot_id, a.shares, p.guarantee_kg
			FROM adoptions a JOIN projects p ON p.id=a.project_id
			WHERE a.id=$1`, in.AdoptionID).
			Scan(&ownerID, &orderProject, &plotID, &shares, &guarantee)
		if err != nil || orderProject != projectID {
			writeErr(w, 404, "认养订单不存在或不属于该批次项目")
			return
		}
		var totalKg float64
		for i, rd := range in.Rounds {
			if rd.EstWeightKg <= 0 {
				writeErr(w, 400, fmt.Sprintf("第%d轮预计重量必须大于0", i+1))
				return
			}
			if strings.TrimSpace(rd.TasteDiff) == "" {
				writeErr(w, 400, fmt.Sprintf("第%d轮必须说明口感差异（糖度/脆面/香气）", i+1))
				return
			}
			totalKg += rd.EstWeightKg
		}
		var active int
		_ = s.Pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM delivery_plans WHERE adoption_id=$1 AND status NOT IN ('archived','cancelled')`,
			in.AdoptionID).Scan(&active)
		if active > 0 {
			writeErr(w, 409, "该订单已有进行中的分批发货方案，请先完成或取消")
			return
		}

		tx, err := s.Pool.Begin(ctx)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		defer tx.Rollback(ctx)

		var planID int64
		err = tx.QueryRow(ctx, `
			INSERT INTO delivery_plans(project_id, adoption_id, batch_no, reason, status, created_by)
			VALUES($1,$2,$3,$4,'draft',$5) RETURNING id`,
			projectID, in.AdoptionID, batchNo, in.Reason, u.ID).Scan(&planID)
		if err != nil {
			writeErr(w, 400, "编制失败："+err.Error())
			return
		}
		for i, rd := range in.Rounds {
			seq := rd.Seq
			if seq == 0 {
				seq = int64(i + 1)
			}
			_, err = tx.Exec(ctx, `
				INSERT INTO delivery_plan_rounds(plan_id, project_id, batch_no, seq, est_weight_kg,
					taste_diff, maturity_note, planned_date, pack_spec, cold_route)
				VALUES($1,$2,$3,$4,$5,$6,$7,NULLIF($8,'')::date,$9,$10)`,
				planID, projectID, batchNo, seq, rd.EstWeightKg, rd.TasteDiff, rd.MaturityNote,
				rd.PlannedDate, rd.PackSpec, rd.ColdRoute)
			if err != nil {
				writeErr(w, 400, "写入发货轮次失败："+err.Error())
				return
			}
		}
		// 权益台账期初保底（权益余额同步认养人）
		need := guarantee * float64(shares)
		_, _ = tx.Exec(ctx, `
			INSERT INTO rights_records(project_id, batch_no, adoption_id, plan_id, kind, change_kg,
				amount, balance_kg, note, operator_id)
			VALUES($1,$2,$3,$4,'opening',$5,0,$5,$6,$7)`,
			projectID, batchNo, in.AdoptionID, planID, need,
			fmt.Sprintf("期初保底权益：%d份 × %.1fkg = %.1fkg（成熟不均，分%d轮发货）",
				shares, guarantee, need, len(in.Rounds)), u.ID)

		if err := tx.Commit(ctx); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		detail, _ := s.planDetail(ctx, planID)
		writeJSON(w, 201, map[string]interface{}{
			"ok":              true,
			"plan":            detail,
			"round_count":     len(in.Rounds),
			"total_weight_kg": totalKg,
		})

	default:
		writeErr(w, 405, "方法不允许")
	}
}

// handleDeliveryPlanByID 处理 /api/delivery-plans/{id}[/{action}]
func (s *Server) handleDeliveryPlanByID(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/delivery-plans/"), "/")
	if rest == "" {
		writeErr(w, 404, "方案不存在")
		return
	}
	parts := strings.Split(rest, "/")
	id, err := parseID(parts[0])
	if err != nil {
		writeErr(w, 404, "方案不存在")
		return
	}
	ctx := r.Context()
	plan, err := s.queryOne(ctx, `
		SELECT dp.*, a.user_id AS adoption_user_id, p.farm_id, p.plot_id,
		       p.guarantee_kg, a.shares
		FROM delivery_plans dp
		JOIN adoptions a ON a.id=dp.adoption_id
		JOIN projects p ON p.id=dp.project_id
		WHERE dp.id=$1`, id)
	if err != nil {
		writeErr(w, 404, "分批发货方案不存在")
		return
	}
	u := s.currentUser(r)
	farmID := asID64(plan["farm_id"])
	onOwnFarm := u.FarmID != nil && *u.FarmID == farmID
	isFarm := u.Role == "admin" || onOwnFarm
	isOwner := asID64(plan["adoption_user_id"]) == u.ID
	canManage := u.Role == "admin" || (u.Role == "farmer" && onOwnFarm) // 农场管理者：提交/归档
	canPack := u.Role == "admin" || ((u.Role == "packer" || u.Role == "farmer") && onOwnFarm)

	if r.Method == http.MethodGet && len(parts) == 1 {
		if !(isOwner || isFarm || u.Role == "service") {
			writeErr(w, 403, "只能查看本农场或本人订单的分批方案")
			return
		}
		detail, e := s.planDetail(ctx, id)
		if e != nil {
			writeErr(w, 500, e.Error())
			return
		}
		writeJSON(w, 200, detail)
		return
	}
	if r.Method != http.MethodPost {
		writeErr(w, 405, "方法不允许")
		return
	}

	// /api/delivery-plans/{id}/rounds/{rid}/picker 安排某一轮的采摘工人
	if len(parts) == 4 && parts[1] == "rounds" && parts[3] == "picker" {
		rid, e := parseID(parts[2])
		if e != nil {
			writeErr(w, 404, "轮次不存在")
			return
		}
		s.assignRoundPicker(w, r, id, rid, plan)
		return
	}
	if len(parts) != 2 {
		writeErr(w, 404, "未知操作")
		return
	}

	switch parts[1] {
	case "submit":
		if !canManage {
			writeErr(w, 403, "只有农场主可以提交采摘计划")
			return
		}
		s.advancePlanStatus(w, r, id, "draft", "submitted",
			`status='submitted', submitted_at=now()`, "采摘计划已同步包装组，等待包装组确认")
	case "pack-confirm":
		if !canPack {
			writeErr(w, 403, "只有本农场包装组/农场主可以确认采摘计划")
			return
		}
		s.advancePlanStatus(w, r, id, "submitted", "pack_confirmed",
			`status='pack_confirmed', packer_id=$2, pack_confirmed_at=now()`,
			"包装组已确认采摘计划，等待认养人确认", u.ID)
	case "confirm":
		if !(isOwner || u.Role == "service" || u.Role == "admin") {
			writeErr(w, 403, "只能由认养本人确认分批方案（客服可代确认）")
			return
		}
		s.adopterConfirmPlan(w, r, plan)
	case "archive":
		if !(canManage || u.Role == "service") {
			writeErr(w, 403, "只有农场主/客服可以归档权益")
			return
		}
		s.archivePlan(w, r, id, plan)
	default:
		writeErr(w, 404, "未知操作")
	}
}

// advancePlanStatus 前置状态校验与通用推进
func (s *Server) advancePlanStatus(w http.ResponseWriter, r *http.Request, planID int64,
	from, to, setClause, okMsg string, extraArgs ...interface{}) {
	ctx := r.Context()
	var cur string
	_ = s.Pool.QueryRow(ctx, `SELECT status FROM delivery_plans WHERE id=$1`, planID).Scan(&cur)
	if cur != from {
		writeErr(w, 409, fmt.Sprintf("方案当前为「%s」，不能执行该操作（需先处于「%s」）",
			planStatusLabel(cur), planStatusLabel(from)))
		return
	}
	q := `UPDATE delivery_plans SET ` + setClause + ` WHERE id=$1`
	args := append([]interface{}{planID}, extraArgs...)
	if _, err := s.Pool.Exec(ctx, q, args...); err != nil {
		writeErr(w, 400, "推进失败："+err.Error())
		return
	}
	detail, _ := s.planDetail(ctx, planID)
	writeJSON(w, 200, map[string]interface{}{"ok": true, "status": to, "message": okMsg, "plan": detail})
}

// adopterConfirmPlan 认养人确认：同步生成每轮物流计划、采摘任务与权益核算
func (s *Server) adopterConfirmPlan(w http.ResponseWriter, r *http.Request, plan map[string]interface{}) {
	ctx := r.Context()
	planID := asID64(plan["id"])
	if plan["status"].(string) != "pack_confirmed" {
		writeErr(w, 409, "包装组尚未确认采摘计划，认养人暂不能确认")
		return
	}
	u := s.currentUser(r)
	var in map[string]interface{}
	_ = decode(r, &in)

	projectID := asID64(plan["project_id"])
	adoptionID := asID64(plan["adoption_id"])
	batchNo, _ := plan["batch_no"].(string)
	plotID := asID64(plan["plot_id"])
	guarantee, _ := plan["guarantee_kg"].(float64)
	shares := asID64(plan["shares"])
	need := guarantee * float64(shares)

	rounds, err := s.queryRows(ctx,
		`SELECT * FROM delivery_plan_rounds WHERE plan_id=$1 ORDER BY seq`, planID)
	if err != nil || len(rounds) == 0 {
		writeErr(w, 400, "方案轮次缺失")
		return
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer tx.Rollback(ctx)

	var totalKg float64
	for _, rd := range rounds {
		rid := asID64(rd["id"])
		weight, _ := rd["est_weight_kg"].(float64)
		seq := asID64(rd["seq"])
		planDate := rd["planned_date"]
		packSpec, _ := rd["pack_spec"].(string)
		coldRoute, _ := rd["cold_route"].(string)
		totalKg += weight

		var taskID int64
		err = tx.QueryRow(ctx, `
			INSERT INTO harvest_tasks(project_id, batch_no, plot_id, picker_id, priority, planned_date,
				target_kg, status, plan_round_id)
			SELECT $1,$2,$3,NULL,$4,COALESCE($5::date,p.est_mature_date),$6,'planned',$7
			FROM projects p WHERE p.id=$1 RETURNING id`,
			projectID, batchNo, plotID, seq, planDate, weight, rid).Scan(&taskID)
		if err != nil {
			writeErr(w, 400, "同步采摘任务失败："+err.Error())
			return
		}
		var shipID int64
		err = tx.QueryRow(ctx, `
			INSERT INTO shipments(project_id, adoption_id, batch_no, seq, plan_date, pack_spec,
				cold_route, weight_kg, status)
			VALUES($1,$2,$3,$4,$5::date,$6,$7,$8,'planned') RETURNING id`,
			projectID, adoptionID, batchNo, seq, planDate, packSpec, coldRoute, weight).Scan(&shipID)
		if err != nil {
			writeErr(w, 400, "同步物流计划失败："+err.Error())
			return
		}
		_, _ = tx.Exec(ctx,
			`UPDATE delivery_plan_rounds SET shipment_id=$1, harvest_task_id=$2 WHERE id=$3`,
			shipID, taskID, rid)
		_, _ = tx.Exec(ctx, `
			INSERT INTO rights_records(project_id, batch_no, adoption_id, plan_id, round_id, kind,
				change_kg, balance_kg, note, operator_id)
			VALUES($1,$2,$3,$4,$5,'allocation',0,$6,$7,$8)`,
			projectID, batchNo, adoptionID, planID, rid, need,
			fmt.Sprintf("第%d轮排产 %.1fkg（%s），物流计划与采摘任务已同步", seq, weight, packSpec), u.ID)
	}
	_, _ = tx.Exec(ctx,
		`UPDATE delivery_plans SET status='adopter_confirmed', adopter_confirmed_at=now() WHERE id=$1`, planID)
	_, _ = tx.Exec(ctx, `UPDATE adoptions SET status='shipping' WHERE id=$1 AND status='paid'`, adoptionID)
	_ = asString(in["note"])

	if err := tx.Commit(ctx); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	detail, _ := s.planDetail(ctx, planID)
	writeJSON(w, 200, map[string]interface{}{
		"ok":              true,
		"status":          "adopter_confirmed",
		"round_count":     len(rounds),
		"total_weight_kg": totalKg,
		"message":         "已确认：物流计划、采摘任务与权益核算已同步更新",
		"plan":            detail,
	})
}

// assignRoundPicker 农场为某一轮安排采摘工人（据成熟度分批组织下一次采摘）
func (s *Server) assignRoundPicker(w http.ResponseWriter, r *http.Request, planID, roundID int64, plan map[string]interface{}) {
	u := s.currentUser(r)
	farmID := asID64(plan["farm_id"])
	if !(u.Role == "farmer" || u.Role == "admin") {
		writeErr(w, 403, "只有农场方可以安排采摘工人")
		return
	}
	if u.Role == "farmer" && (u.FarmID == nil || *u.FarmID != farmID) {
		writeErr(w, 403, "只能安排本农场的采摘任务")
		return
	}
	var in struct {
		PickerID    int64  `json:"picker_id"`
		PlannedDate string `json:"planned_date"`
	}
	_ = decode(r, &in)
	ctx := r.Context()
	var taskID int64
	err := s.Pool.QueryRow(ctx,
		`SELECT harvest_task_id FROM delivery_plan_rounds WHERE id=$1 AND plan_id=$2`, roundID, planID).
		Scan(&taskID)
	if err != nil {
		writeErr(w, 404, "轮次不存在")
		return
	}
	if taskID == 0 {
		writeErr(w, 409, "认养人尚未确认方案，采摘任务还未生成")
		return
	}
	if in.PickerID > 0 {
		var role string
		if e := s.Pool.QueryRow(ctx, `SELECT role FROM users WHERE id=$1`, in.PickerID).Scan(&role); e != nil {
			writeErr(w, 404, "采摘工不存在")
			return
		}
		if role != "picker" {
			writeErr(w, 400, "被指派人不是采摘工账号")
			return
		}
	}
	q := `UPDATE harvest_tasks SET picker_id=COALESCE(NULLIF($2,0),picker_id)`
	args := []interface{}{taskID, in.PickerID}
	if in.PlannedDate != "" {
		q += `, planned_date=$3::date`
		args = append(args, in.PlannedDate)
	}
	q += ` WHERE id=$1 RETURNING *`
	row, err := s.queryOne(ctx, q, args...)
	if err != nil {
		writeErr(w, 400, "安排失败："+err.Error())
		return
	}
	writeJSON(w, 200, map[string]interface{}{"ok": true, "harvest_task": row})
}

// archivePlan 权益归档：所有轮次已送达、权益余额为零
func (s *Server) archivePlan(w http.ResponseWriter, r *http.Request, planID int64, plan map[string]interface{}) {
	ctx := r.Context()
	if plan["status"].(string) != "adopter_confirmed" {
		writeErr(w, 409, "方案尚未经认养人确认，不能归档")
		return
	}
	adoptionID := asID64(plan["adoption_id"])
	projectID := asID64(plan["project_id"])
	batchNo, _ := plan["batch_no"].(string)

	var undelivered, noShip int
	_ = s.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM delivery_plan_rounds r
		JOIN shipments sh ON sh.id=r.shipment_id
		WHERE r.plan_id=$1 AND sh.status<>'delivered'`, planID).Scan(&undelivered)
	_ = s.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM delivery_plan_rounds WHERE plan_id=$1 AND shipment_id IS NULL`,
		planID).Scan(&noShip)
	if undelivered > 0 || noShip > 0 {
		writeErr(w, 409, "仍有轮次未送达，权益余额尚未兑现完毕，不能归档")
		return
	}
	remaining, err := s.remainingRights(ctx, adoptionID)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if remaining > 0.01 {
		writeErr(w, 409, fmt.Sprintf("权益余额还剩 %.2fkg 未兑现（需补发或赔付折算后）才能归档", remaining))
		return
	}
	var in map[string]interface{}
	_ = decode(r, &in)
	u := s.currentUser(r)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer tx.Rollback(ctx)
	_, _ = tx.Exec(ctx,
		`UPDATE delivery_plans SET status='archived', archived_at=now(), archive_note=$2 WHERE id=$1`,
		planID, asString(in["archive_note"]))
	_, _ = tx.Exec(ctx, `
		INSERT INTO rights_records(project_id, batch_no, adoption_id, plan_id, kind, change_kg,
			balance_kg, note, archived, operator_id)
		VALUES($1,$2,$3,$4,'archive',0,0,$5,TRUE,$6)`,
		projectID, batchNo, adoptionID, planID, "全部轮次送达、权益余额为零，权益台账归档", u.ID)
	_, _ = tx.Exec(ctx, `UPDATE rights_records SET archived=TRUE WHERE adoption_id=$1`, adoptionID)
	if err := tx.Commit(ctx); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	detail, _ := s.planDetail(ctx, planID)
	writeJSON(w, 200, map[string]interface{}{"ok": true, "status": "archived", "message": "权益已归档", "plan": detail})
}

// ---------- 查询与权益核算 ----------

// planDetail 组装方案头、各轮（含运单/采摘任务）、权益台账与权益余额汇总
func (s *Server) planDetail(ctx context.Context, planID int64) (map[string]interface{}, error) {
	plan, err := s.queryOne(ctx, `
		SELECT dp.*, a.recipient_name, a.user_id AS adoption_user_id, a.shares,
		       p.title AS project_title, p.kind,
		       au.name AS adopter_name, pk.name AS packer_name
		FROM delivery_plans dp
		JOIN adoptions a ON a.id=dp.adoption_id
		JOIN projects p ON p.id=dp.project_id
		JOIN users au ON au.id=a.user_id
		LEFT JOIN users pk ON pk.id=dp.packer_id
		WHERE dp.id=$1`, planID)
	if err != nil {
		return nil, err
	}
	rounds, err := s.queryRows(ctx, `
		SELECT r.*, sh.status AS shipment_status, sh.tracking_no, sh.shipped_at, sh.delivered_at,
		       sh.bad_fruit_rate,
		       ht.status AS harvest_status, ht.actual_kg, ht.picker_id, pu.name AS picker_name
		FROM delivery_plan_rounds r
		LEFT JOIN shipments sh ON sh.id=r.shipment_id
		LEFT JOIN harvest_tasks ht ON ht.id=r.harvest_task_id
		LEFT JOIN users pu ON pu.id=ht.picker_id
		WHERE r.plan_id=$1 ORDER BY r.seq`, planID)
	if err != nil {
		return nil, err
	}
	rights, _ := s.queryRows(ctx,
		`SELECT rr.*, u.name AS operator_name FROM rights_records rr
		 LEFT JOIN users u ON u.id=rr.operator_id
		 WHERE rr.plan_id=$1 ORDER BY rr.id`, planID)

	adoptionID := asID64(plan["adoption_id"])
	summary, _ := s.adoptionRightsSummary(ctx, adoptionID)
	plan["status_label"] = planStatusLabel(asString(plan["status"]))
	plan["rounds"] = rounds
	plan["rights_records"] = rights
	plan["rights_summary"] = summary
	return plan, nil
}

// adoptionRightsSummary 认养订单权益余额：已送达 / 在途 / 待发 / 赔付折算 / 待兑现余额
func (s *Server) adoptionRightsSummary(ctx context.Context, adoptionID int64) (map[string]interface{}, error) {
	var shares int64
	var guarantee, unitPrice float64
	err := s.Pool.QueryRow(ctx, `
		SELECT a.shares, p.guarantee_kg, a.unit_price
		FROM adoptions a JOIN projects p ON p.id=a.project_id WHERE a.id=$1`,
		adoptionID).Scan(&shares, &guarantee, &unitPrice)
	if err != nil {
		return nil, err
	}
	need := guarantee * float64(shares)

	var delivered, inTransit, pending float64
	stRows, _ := s.queryRows(ctx,
		`SELECT status, COALESCE(SUM(weight_kg),0) AS kg FROM shipments WHERE adoption_id=$1 GROUP BY status`,
		adoptionID)
	for _, r0 := range stRows {
		kg, _ := r0["kg"].(float64)
		switch r0["status"] {
		case "delivered":
			delivered += kg
		case "shipped":
			inTransit += kg
		default:
			pending += kg
		}
	}
	var compKg, compAmount float64
	comps, _ := s.queryRows(ctx,
		`SELECT amount, kind FROM compensations WHERE adoption_id=$1 AND status='paid'`, adoptionID)
	if unitPrice > 0 && guarantee > 0 {
		for _, c := range comps {
			amt, _ := c["amount"].(float64)
			kind, _ := c["kind"].(string)
			compAmount += amt
			if kind == "reship" {
				compKg += amt / (unitPrice / guarantee)
			} else {
				compKg += amt / unitPrice * guarantee
			}
		}
	}
	remaining := maxF(need-delivered-compKg, 0)
	return map[string]interface{}{
		"guarantee_kg":         need,
		"delivered_kg":         delivered,
		"in_transit_kg":        inTransit,
		"pending_kg":           pending,
		"compensated_kg_equiv": roundF(compKg, 2),
		"compensated_amount":   compAmount,
		"remaining_kg":         roundF(remaining, 2),
	}, nil
}

// remainingRights 待兑现权益余额（保底 - 已送达 - 赔付折算）
func (s *Server) remainingRights(ctx context.Context, adoptionID int64) (float64, error) {
	summary, err := s.adoptionRightsSummary(ctx, adoptionID)
	if err != nil {
		return 0, err
	}
	v, _ := summary["remaining_kg"].(float64)
	return v, nil
}

// recordShipmentRights 运单发车/送达时写权益台账（仅对挂在分轮方案上的运单）
func (s *Server) recordShipmentRights(ctx context.Context, shipID int64, to string, operatorID int64) {
	var roundID, planID, adoptionID int64
	var seq int64
	var weight float64
	err := s.Pool.QueryRow(ctx, `
		SELECT r.id, r.plan_id, sh.adoption_id, r.seq, COALESCE(sh.weight_kg,0)
		FROM shipments sh JOIN delivery_plan_rounds r ON r.shipment_id=sh.id
		WHERE sh.id=$1`, shipID).Scan(&roundID, &planID, &adoptionID, &seq, &weight)
	if err != nil {
		return // 非成熟度分批方案运单，不写权益流水
	}
	var projectID int64
	var batchNo string
	_ = s.Pool.QueryRow(ctx, `SELECT project_id, batch_no FROM delivery_plans WHERE id=$1`, planID).
		Scan(&projectID, &batchNo)
	remaining, _ := s.remainingRights(ctx, adoptionID)
	switch to {
	case "shipped":
		_, _ = s.Pool.Exec(ctx, `
			INSERT INTO rights_records(project_id, batch_no, adoption_id, plan_id, round_id, kind,
				change_kg, balance_kg, note, operator_id)
			VALUES($1,$2,$3,$4,$5,'dispatch',0,$6,$7,$8)`,
			projectID, batchNo, adoptionID, planID, roundID, remaining,
			fmt.Sprintf("第%d轮冷链发车，%.1fkg 在途（权益余额含在途）", seq, weight), operatorID)
	case "delivered":
		_, _ = s.Pool.Exec(ctx, `
			INSERT INTO rights_records(project_id, batch_no, adoption_id, plan_id, round_id, kind,
				change_kg, balance_kg, note, operator_id)
			VALUES($1,$2,$3,$4,$5,'delivery',$6,$7,$8,$9)`,
			projectID, batchNo, adoptionID, planID, roundID, -weight, remaining,
			fmt.Sprintf("第%d轮送达签收，兑现 %.1fkg，权益余额同步更新", seq, weight), operatorID)
	}
}

func asID64(v interface{}) int64 {
	id, _ := asID(v)
	return id
}

func planStatusLabel(st string) string {
	switch st {
	case "draft":
		return "编制中"
	case "submitted":
		return "待包装组确认"
	case "pack_confirmed":
		return "待认养人确认"
	case "adopter_confirmed":
		return "认养人已确认/履约中"
	case "archived":
		return "已归档"
	case "cancelled":
		return "已取消"
	}
	return st
}
