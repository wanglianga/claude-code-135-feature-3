package api

import (
	"context"
	"fmt"
	"net/http"
)

// ---------- 成熟度分批发货方案 ----------

// handleRoundPlans POST 平台/农场按成熟不均把发货拆成多轮（每轮预计重量 + 口感差异）
func (s *Server) handleRoundPlans(w http.ResponseWriter, r *http.Request, projectID int64, batchNo string, ownFarm bool) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, "方法不允许")
		return
	}
	if !ownFarm {
		writeErr(w, 403, "只有农场方可以编排名成熟度分批发货方案")
		return
	}
	u := s.currentUser(r)
	var in struct {
		AdoptionID int64  `json:"adoption_id"`
		Reason     string `json:"reason"`
		Rounds     []struct {
			Seq         int64   `json:"seq"`
			PlanDate    string  `json:"plan_date"`
			EstWeightKg float64 `json:"est_weight_kg"`
			TasteDiff   string  `json:"taste_diff"`
			PackSpec    string  `json:"pack_spec"`
			ColdRoute   string  `json:"cold_route"`
		} `json:"rounds"`
	}
	if err := decode(r, &in); err != nil || in.AdoptionID == 0 {
		writeErr(w, 400, "adoption_id 必填")
		return
	}
	if len(in.Rounds) == 0 {
		writeErr(w, 400, "至少编排一轮发货")
		return
	}
	for i, rd := range in.Rounds {
		if rd.Seq == 0 {
			in.Rounds[i].Seq = int64(i + 1)
		}
		if rd.EstWeightKg <= 0 {
			writeErr(w, 400, fmt.Sprintf("第 %d 轮预计重量必须大于 0", in.Rounds[i].Seq))
			return
		}
		if rd.TasteDiff == "" {
			writeErr(w, 400, fmt.Sprintf("第 %d 轮必须说明口感差异", in.Rounds[i].Seq))
			return
		}
		if rd.PlanDate == "" {
			writeErr(w, 400, fmt.Sprintf("第 %d 轮计划日期必填", in.Rounds[i].Seq))
			return
		}
	}
	// 校验订单属于本项目
	var owner int64
	err := s.Pool.QueryRow(r.Context(),
		`SELECT user_id FROM adoptions WHERE id=$1 AND project_id=$2`, in.AdoptionID, projectID).Scan(&owner)
	if err != nil {
		writeErr(w, 404, "该认养订单不存在或不属于本批次")
		return
	}
	// 同一订单已有待确认方案时不允许重复发起
	var n int
	_ = s.Pool.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM shipment_round_plans WHERE adoption_id=$1 AND status='proposed'`, in.AdoptionID).Scan(&n)
	if n > 0 {
		writeErr(w, 409, "该订单已有待认养人确认的分轮方案")
		return
	}

	ctx := r.Context()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer tx.Rollback(ctx)
	var planID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO shipment_round_plans(project_id, batch_no, adoption_id, reason, status, created_by)
		VALUES($1,$2,$3,$4,'proposed',$5) RETURNING id`,
		projectID, batchNo, in.AdoptionID, in.Reason, u.ID).Scan(&planID)
	if err != nil {
		writeErr(w, 400, "编排失败："+err.Error())
		return
	}
	for _, rd := range in.Rounds {
		_, err = tx.Exec(ctx, `
			INSERT INTO shipment_round_items(plan_id, project_id, adoption_id, seq, plan_date,
				est_weight_kg, taste_diff, pack_spec, cold_route)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			planID, projectID, in.AdoptionID, rd.Seq, rd.PlanDate, rd.EstWeightKg,
			rd.TasteDiff, rd.PackSpec, rd.ColdRoute)
		if err != nil {
			writeErr(w, 400, "写入轮次失败："+err.Error())
			return
		}
	}
	if err := tx.Commit(ctx); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, s.roundPlanDetail(r.Context(), planID))
}

// handleRoundConfirm 认养人确认分轮方案：同步生成各轮物流计划（planned）并重算权益核算
func (s *Server) handleRoundConfirm(w http.ResponseWriter, r *http.Request, projectID, planID int64, reject bool) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, "方法不允许")
		return
	}
	u := s.currentUser(r)
	var in map[string]interface{}
	_ = decode(r, &in)
	ctx := r.Context()

	var adoptionID, owner int64
	var status, batchNo string
	err := s.Pool.QueryRow(ctx,
		`SELECT adoption_id, status, batch_no FROM shipment_round_plans WHERE id=$1 AND project_id=$2`,
		planID, projectID).Scan(&adoptionID, &status, &batchNo)
	if err != nil {
		writeErr(w, 404, "分轮方案不存在")
		return
	}
	if status != "proposed" {
		writeErr(w, 409, "该方案已处理，当前状态："+status)
		return
	}
	if err := s.Pool.QueryRow(ctx, `SELECT user_id FROM adoptions WHERE id=$1`, adoptionID).Scan(&owner); err != nil {
		writeErr(w, 404, "认养订单不存在")
		return
	}
	if u.ID != owner && u.Role != "service" && u.Role != "admin" {
		writeErr(w, 403, "只能由认养人本人确认发货方案")
		return
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer tx.Rollback(ctx)

	if reject {
		note := asString(in["note"])
		_, err = tx.Exec(ctx,
			`UPDATE shipment_round_plans SET status='rejected' WHERE id=$1`, planID)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		// 拒绝原因挂到批次工单流水，便于农场跟进重排
		if note != "" {
			var pid int64
			var batch string
			_ = tx.QueryRow(ctx, `SELECT project_id, batch_no FROM shipment_round_plans WHERE id=$1`, planID).Scan(&pid, &batch)
			var iid int64
			if e := tx.QueryRow(ctx, `
				INSERT INTO incidents(project_id, batch_no, adoption_id, kind, title, description, reporter_id, status)
				VALUES($1,$2,$3,'delivery_dispute','认养人未接受分批发货方案',$4,$5,'open') RETURNING id`,
				pid, batch, adoptionID, note, u.ID).Scan(&iid); e == nil {
				_, _ = tx.Exec(ctx, `
					INSERT INTO incident_events(incident_id, actor_id, actor_role, action, note)
					VALUES($1,$2,'adopter','拒绝分轮方案',$3)`, iid, u.ID, note)
			}
		}
		if err := tx.Commit(ctx); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]interface{}{"ok": true, "status": "rejected"})
		return
	}

	_, err = tx.Exec(ctx,
		`UPDATE shipment_round_plans SET status='confirmed', confirmed_by=$1, confirmed_at=now() WHERE id=$2`,
		u.ID, planID)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	// 逐轮生成/关联物流计划；已存在关联运单的轮次跳过
	rows, err := tx.Query(ctx,
		`SELECT id, seq, TO_CHAR(plan_date,'YYYY-MM-DD'), est_weight_kg, pack_spec, cold_route, COALESCE(shipment_id,0)
		 FROM shipment_round_items WHERE plan_id=$1 ORDER BY seq`, planID)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	type item struct {
		id, seq, shipID   int64
		date, spec, route string
		weight            float64
	}
	items := []item{}
	for rows.Next() {
		var it item
		if e := rows.Scan(&it.id, &it.seq, &it.date, &it.weight, &it.spec, &it.route, &it.shipID); e != nil {
			rows.Close()
			writeErr(w, 500, e.Error())
			return
		}
		items = append(items, it)
	}
	rows.Close()
	for _, it := range items {
		if it.shipID != 0 {
			continue
		}
		var sid int64
		err = tx.QueryRow(ctx, `
			INSERT INTO shipments(project_id, adoption_id, batch_no, seq, plan_date, pack_spec, cold_route,
			                      weight_kg, status, round_plan_id, round_item_id)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,'planned',$9,$10)
			RETURNING id`,
			projectID, adoptionID, batchNo, it.seq, nilIfEmpty(it.date), it.spec, it.route, it.weight,
			planID, it.id).Scan(&sid)
		if err != nil {
			writeErr(w, 400, "同步物流计划失败："+err.Error())
			return
		}
		_, _ = tx.Exec(ctx, `UPDATE shipment_round_items SET shipment_id=$1 WHERE id=$2`, sid, it.id)
	}
	if err := tx.Commit(ctx); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	// 确认后物流计划与权益核算同步更新
	if e := s.recomputeRightsArchive(context.Background(), adoptionID); e != nil {
		// 权益重算失败不阻断确认，仅记录
		fmt.Println("recompute rights error:", e)
	}
	writeJSON(w, 200, s.roundPlanDetail(r.Context(), planID))
}

// roundPlanDetail 返回方案 + 轮次明细
func (s *Server) roundPlanDetail(ctx context.Context, planID int64) map[string]interface{} {
	plan, err := s.queryOne(ctx, `
		SELECT rp.*, cu.name AS confirmed_name, au.name AS adopter_name
		FROM shipment_round_plans rp
		LEFT JOIN users cu ON cu.id=rp.confirmed_by
		LEFT JOIN adoptions a ON a.id=rp.adoption_id
		LEFT JOIN users au ON au.id=a.user_id
		WHERE rp.id=$1`, planID)
	if err != nil {
		return map[string]interface{}{"id": planID}
	}
	items, _ := s.queryRows(ctx,
		`SELECT ri.*, sh.status AS shipment_status, sh.tracking_no
		 FROM shipment_round_items ri
		 LEFT JOIN shipments sh ON sh.id=ri.shipment_id
		 WHERE ri.plan_id=$1 ORDER BY ri.seq`, planID)
	plan["items"] = items
	return plan
}

// ---------- 采摘计划（农场安排下一次采摘工人 → 同步包装组确认） ----------

// handleHarvestPlansSub POST 农场据分轮方案安排下一次采摘工人
func (s *Server) handleHarvestPlansSub(w http.ResponseWriter, r *http.Request, projectID int64, batchNo string, ownFarm bool) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, "方法不允许")
		return
	}
	if !ownFarm {
		writeErr(w, 403, "只有农场方可以安排采摘计划")
		return
	}
	u := s.currentUser(r)
	var in struct {
		RoundPlanID int64   `json:"round_plan_id"`
		RoundSeq    int64   `json:"round_seq"`
		PickerID    int64   `json:"picker_id"`
		PlannedDate string  `json:"planned_date"`
		TargetKg    float64 `json:"target_kg"`
		WorkerNote  string  `json:"worker_note"`
		FruitNote   string  `json:"fruit_note"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, 400, "请求格式错误")
		return
	}
	if in.PlannedDate == "" || in.TargetKg <= 0 {
		writeErr(w, 400, "planned_date 与 target_kg 必填")
		return
	}
	row, err := s.insertRow(r.Context(), "harvest_plans", map[string]interface{}{
		"project_id":    projectID,
		"batch_no":      batchNo,
		"round_plan_id": nullableID(in.RoundPlanID),
		"round_seq":     in.RoundSeq,
		"picker_id":     nullableID(in.PickerID),
		"planned_date":  in.PlannedDate,
		"target_kg":     in.TargetKg,
		"worker_note":   in.WorkerNote,
		"fruit_note":    in.FruitNote,
		"created_by":    u.ID,
	})
	if err != nil {
		writeErr(w, 400, "安排失败："+err.Error())
		return
	}
	writeJSON(w, 201, row)
}

// handleHarvestPlanConfirm 包装组确认采摘计划：确认后生成采摘任务（采摘工人可见、可回填）
func (s *Server) handleHarvestPlanConfirm(w http.ResponseWriter, r *http.Request, projectID, hpID int64, ownFarm bool) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, "方法不允许")
		return
	}
	u := s.currentUser(r)
	// 采摘计划由农场安排，必须由包装组确认（管理员可代录）；安排人（农场/采摘工）不能自行确认
	if u.Role != "packer" && u.Role != "admin" {
		writeErr(w, 403, "只有包装组可以确认采摘计划")
		return
	}
	var in map[string]interface{}
	_ = decode(r, &in)
	ctx := r.Context()

	var pickerID, plotID, taskID int64
	var status, plannedDate, batchNo, workerNote, fruitNote string
	var roundSeq int64
	var targetKg float64
	err := s.Pool.QueryRow(ctx, `
		SELECT COALESCE(hp.picker_id,0), p.plot_id, hp.status, TO_CHAR(hp.planned_date,'YYYY-MM-DD'), hp.target_kg,
		       COALESCE(hp.harvest_task_id,0), COALESCE(hp.round_seq,0), hp.batch_no,
		       COALESCE(hp.worker_note,''), COALESCE(hp.fruit_note,'')
		FROM harvest_plans hp JOIN projects p ON p.id=hp.project_id
		WHERE hp.id=$1 AND hp.project_id=$2`, hpID, projectID).
		Scan(&pickerID, &plotID, &status, &plannedDate, &targetKg, &taskID, &roundSeq, &batchNo,
			&workerNote, &fruitNote)
	if err != nil {
		writeErr(w, 404, "采摘计划不存在")
		return
	}
	if status != "planned" {
		writeErr(w, 409, "该采摘计划已处理，当前状态："+status)
		return
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer tx.Rollback(ctx)

	priority := 100
	if roundSeq > 0 {
		priority = int(roundSeq) * 10
	}
	if taskID == 0 {
		err = tx.QueryRow(ctx, `
			INSERT INTO harvest_tasks(project_id, batch_no, plot_id, picker_id, priority, planned_date, target_kg)
			VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
			projectID, batchNo, plotID, nullableID(pickerID), priority, plannedDate, targetKg).Scan(&taskID)
		if err != nil {
			writeErr(w, 400, "生成采摘任务失败："+err.Error())
			return
		}
	}
	note := asString(in["note"])
	_, err = tx.Exec(ctx, `
		UPDATE harvest_plans SET status='confirmed', packer_id=$1, packer_note=$2,
		                         confirmed_at=now(), harvest_task_id=$3
		WHERE id=$4`, u.ID, note, taskID, hpID)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if err := tx.Commit(ctx); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	row, _ := s.queryOne(ctx, `
		SELECT hp.*, pu.name AS picker_name, ku.name AS packer_name
		FROM harvest_plans hp
		LEFT JOIN users pu ON pu.id=hp.picker_id
		LEFT JOIN users ku ON ku.id=hp.packer_id
		WHERE hp.id=$1`, hpID)
	writeJSON(w, 200, row)
}

// ---------- 权益归档 / 余额 ----------

// recomputeRightsArchive 按最新物流计划与赔付重算认养人权益余额，全部送达后归档
func (s *Server) recomputeRightsArchive(ctx context.Context, adoptionID int64) error {
	var pid int64
	var batchNo string
	var shares int64
	var unitPrice, guarantee float64
	err := s.Pool.QueryRow(ctx, `
		SELECT a.project_id, a.batch_no, a.shares, a.unit_price, p.guarantee_kg
		FROM adoptions a JOIN projects p ON p.id=a.project_id
		WHERE a.id=$1`, adoptionID).
		Scan(&pid, &batchNo, &shares, &unitPrice, &guarantee)
	if err != nil {
		return err
	}
	need := guarantee * float64(shares)

	var shipped, pending float64
	shipRows, err := s.queryRows(ctx,
		`SELECT weight_kg, status FROM shipments WHERE adoption_id=$1`, adoptionID)
	if err != nil {
		return err
	}
	hasShip := false
	allDelivered := true
	for _, sh := range shipRows {
		w, _ := sh["weight_kg"].(float64)
		hasShip = true
		if sh["status"] == "planned" {
			pending += w
			allDelivered = false
		} else {
			shipped += w
			if sh["status"] != "delivered" {
				allDelivered = false
			}
		}
	}
	if !hasShip {
		allDelivered = false
	}

	var compKg, compAmount float64
	if unitPrice > 0 && guarantee > 0 {
		comps, _ := s.queryRows(ctx,
			`SELECT amount, kind FROM compensations WHERE adoption_id=$1 AND status='paid'`, adoptionID)
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
	balance := need - shipped - pending - compKg
	if balance < 0 {
		balance = 0
	}
	status := "active"
	summary := ""
	if hasShip && allDelivered {
		status = "archived"
		summary = fmt.Sprintf(
			"权益已归档：保底 %.1fkg，累计已发 %.1fkg，赔付折算 %.2fkg，权益余额 %.1fkg，全部批次已送达。",
			need, shipped, compKg, balance)
	} else {
		summary = fmt.Sprintf(
			"保底 %.1fkg；已发 %.1fkg，待发 %.1fkg，赔付折算 %.2fkg，权益余额 %.1fkg。",
			need, shipped, pending, compKg, balance)
	}

	_, err = s.Pool.Exec(ctx, `
		INSERT INTO rights_archives(adoption_id, project_id, batch_no, guarantee_kg, shipped_kg,
			pending_kg, compensated_kg_equiv, balance_kg, status, summary, synced_at, archived_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,now(), CASE WHEN $9='archived' THEN now() ELSE NULL END)
		ON CONFLICT (adoption_id) DO UPDATE SET
			project_id=EXCLUDED.project_id, batch_no=EXCLUDED.batch_no,
			guarantee_kg=EXCLUDED.guarantee_kg, shipped_kg=EXCLUDED.shipped_kg,
			pending_kg=EXCLUDED.pending_kg, compensated_kg_equiv=EXCLUDED.compensated_kg_equiv,
			balance_kg=EXCLUDED.balance_kg, status=EXCLUDED.status, summary=EXCLUDED.summary,
			synced_at=now(),
			archived_at=CASE WHEN EXCLUDED.status='archived' THEN COALESCE(rights_archives.archived_at, now())
			                 ELSE rights_archives.archived_at END`,
		adoptionID, pid, batchNo, need, shipped, pending, roundF(compKg, 2), balance, status, summary)
	return err
}

// handleMyRights GET 认养人的权益余额与归档（已发/待发/权益余额）
func (s *Server) handleMyRights(w http.ResponseWriter, r *http.Request) {
	u := s.currentUser(r)
	ctx := r.Context()
	rows, err := s.queryRows(ctx, `
		SELECT a.id AS adoption_id, a.shares, a.status AS adoption_status,
		       p.title, p.kind, p.batch_no, p.guarantee_kg, a.unit_price
		FROM adoptions a JOIN projects p ON p.id=a.project_id
		WHERE a.user_id=$1 ORDER BY a.id DESC`, u.ID)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	out := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		aid, _ := row["adoption_id"].(int64)
		// 实时重算并落库，保证认养人看到的余额与物流/赔付同步
		if e := s.recomputeRightsArchive(ctx, aid); e != nil {
			fmt.Println("recompute rights error:", e)
		}
		arch, _ := s.queryOne(ctx, `SELECT * FROM rights_archives WHERE adoption_id=$1`, aid)
		plans, _ := s.queryRows(ctx, `
			SELECT rp.id, rp.status, rp.reason, rp.confirmed_at,
			       (SELECT json_agg(ri.* ORDER BY ri.seq)
			          FROM shipment_round_items ri WHERE ri.plan_id=rp.id) AS items
			FROM shipment_round_plans rp WHERE rp.adoption_id=$1 ORDER BY rp.id`, aid)
		row["rights"] = arch
		row["round_plans"] = plans
		out = append(out, row)
	}
	writeJSON(w, 200, out)
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
