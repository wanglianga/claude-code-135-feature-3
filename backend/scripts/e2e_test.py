#!/usr/bin/env python3
# 端到端 API 验证：覆盖认养付款/证书/农事/灾害/抽检/采摘/分拣/发货状态机/
# 改地址/坏果投诉/权益争议/多角色工单流水/赔付/保险/回访/直播/参观/团购/公益/台账/看板/结算
import json
import os
import sys
import urllib.request
import urllib.error

B = os.environ.get('FARMCROWD_BASE', 'http://127.0.0.1:8080')
results = []

def call(method, path, token=None, body=None):
    url = B + path
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req) as r:
            return r.status, json.loads(r.read().decode())
    except urllib.error.HTTPError as e:
        try:
            return e.code, json.loads(e.read().decode())
        except Exception:
            return e.code, {}

def check(name, cond, detail=""):
    results.append((name, cond, detail))
    print(("PASS" if cond else "FAIL") + ": " + name + ("" if cond else "  -> " + str(detail)[:200]))

def login(phone):
    st, r = call("POST", "/api/auth/login", body={"phone": phone, "password": "farm123"})
    assert st == 200, r
    return r["token"]

T = {
    "adopter":  login("13900000002"),  # 陈曦
    "adopter2": login("13900000008"),  # 吴琳
    "farmer":   login("13900000001"),  # 李远山
    "picker":   login("13900000003"),
    "packer":   login("13900000004"),
    "logistics":login("13900000005"),
    "service":  login("13900000006"),
    "admin":    login("13800000000"),
}
check("7 个角色全部登录成功", all(T.values()))

# ---- 公开浏览 ----
st, projs = call("GET", "/api/projects")
check("公开浏览 5 个认养项目", st == 200 and len(projs) == 5, len(projs))
st, p1 = call("GET", "/api/projects/1")
need_keys = {"project", "logs", "maturity_checks", "harvest_tasks", "sorting",
             "shipments", "incidents", "insurance_claims", "settlement", "delivery_plans"}
check("项目详情聚合全链路", need_keys.issubset(p1.keys()), p1.keys())
check("项目农事日志含冰雹", any("冰雹" in l.get("title", "") for l in p1["logs"]))
check("项目详情含成熟度分批方案与轮次口感差异",
      len(p1["delivery_plans"]) >= 1 and
      all(any(r.get("taste_diff") for r in pl["rounds"]) for pl in p1["delivery_plans"]),
      p1["delivery_plans"])
st, live = call("GET", "/api/live")
check("农场直播公开可见", any("红富士" in x["title"] for x in live))
st, charity = call("GET", "/api/charity")
check("公益认养公开榜", any("敬老院" in x["donee"] for x in charity))
st, _ = call("POST", "/api/adoptions", body={"project_id": 5})
check("未登录认养被拒", st == 401)

# ---- 认养付款（吴琳购买蔬菜 1 份，走全新订单）----
st, r = call("POST", "/api/adoptions", T["adopter2"], {
    "project_id": 5, "shares": 1, "recipient_name": "吴琳", "phone": "13900000008",
    "address": "杭州市西湖区文三路478号华星时代广场"})
check("付款成功并自动生成证书", st == 201 and r["cert_no"].startswith("CERT-VEG-2026-A"), r)
new_aid = r["adoption"]["id"]

st, r = call("POST", "/api/adoptions", T["adopter2"], {
    "project_id": 2, "shares": 999, "recipient_name": "x", "phone": "13900000000", "address": "x"})
check("超卖拦截", st == 409 and "不足" in r.get("error", ""), r)

# ---- 我的认养 · 权益公平兑现（陈曦苹果订单 id=1：保底40kg）----
st, mine = call("GET", "/api/my/adoptions", T["adopter"])
apple = next(a for a in mine if a["kind"] == "apple")
rf = apple["rights_fulfillment"]
print("   苹果权益核对:", rf)
check("保底量=40kg(20×2)", rf["guarantee_kg"] == 40, rf)
check("已发货 20kg（首批12+二批8计划）", rf["shipped_kg"] == 12 and rf["planned_kg"] == 8, rf)
# 补发39.9折1.33kg + 保险560折18.70kg；履约中已实现32kg，计入二批在途8kg后达40kg
check("履约中已实现尚不足(在途批次未发)", rf["fair"] is False, rf)
check("含在途批次后预计公平兑现=true", rf["projected_fair"] is True, rf)
check("赔付含保险560", rf["compensated_amount"] >= 599.9, rf)
check("证书号展示", apple["cert_no"] == "CERT-APPLE-2026-A-0001")
check("分批发货计划与冷链路线", len(apple["shipments"]) == 2 and "冷链车" in apple["shipments"][0]["cold_route"])
check("回访记录可见", len(apple["callbacks"]) == 1 and apple["callbacks"][0]["would_renew"])

# ---- 临时改地址（未发货可改，已发货拦截）----
st, r = call("POST", f"/api/adoptions/{new_aid}/address", T["adopter2"], {
    "recipient_name": "吴父", "phone": "13100000000", "address": "宁波市鄞州区首南街道9号",
    "reason": "出差改寄父母家"})
check("未发货订单改址成功并建工单", st == 200 and r["ok"] and r["incident_id"], r)
st, r = call("POST", "/api/adoptions/1/address", T["adopter"], {"address": "x路1号"})
check("已有发出批次改址被拦截", st == 409, r)

# ---- 农场端：灾害日志自动建工单 ----
st, r = call("POST", "/api/projects/5/logs", T["farmer"], {
    "log_type": "disaster", "title": "暖棚短时大风掀膜",
    "content": "连夜修补，叶菜延后3天上市", "disaster_kind": "大风", "yield_impact_pct": 5})
check("登记灾害农事日志", st == 201 and r["disaster_kind"] == "大风", r)
st, p5 = call("GET", "/api/projects/5")
check("大风灾害自动生成工单并入同一批次",
      any(i["kind"] in ("delay", "other", "hail") and "大风" in i["title"] for i in p5["incidents"]),
      p5["incidents"])

# ---- 成熟度抽检（糖度/分级/坏果率/预估产量）----
st, r = call("POST", "/api/projects/5/checks", T["farmer"], {
    "brix": 6.2, "sample_count": 20, "maturity_pct": 80, "est_yield_kg": 300,
    "bad_rate_pct": 1.2, "grade_dist": {"大": 50, "中": 40, "小": 10}, "notes": "叶菜达标"})
check("成熟度抽检登记", st == 201 and r["maturity_pct"] == 80 and r["grade_dist"]["大"] == 50, r)

# ---- 采摘安排（优先级）+ 采摘工回填 ----
st, r = call("POST", "/api/projects/5/harvests", T["farmer"], {
    "picker_id": 4, "priority": 10, "planned_date": "2026-10-16", "target_kg": 100})
check("农场安排采摘任务", st == 201, r)
hid = r["id"]
st, r = call("POST", f"/api/projects/5/harvests/{hid}/complete", T["picker"],
             {"actual_kg": 96, "bad_rate_pct": 2.1})
check("采摘工回填实际产量与坏果率", st == 200 and r["status"] == "done" and r["actual_kg"] == 96, r)
st, r = call("POST", f"/api/projects/5/harvests/{hid}/complete", T["adopter"],
             {"actual_kg": 1, "bad_rate_pct": 0})
check("非授权人员不能回填采摘", st == 403, r)

# ---- 分拣（包装规格）----
st, r = call("POST", "/api/projects/5/sorting", T["packer"],
             {"spec": "有机叶菜标准箱 8斤", "weight_kg": 96, "box_count": 24})
check("包装员登记分拣规格", st == 201 and r["box_count"] == 24, r)

# ---- 发货状态机 ----
st, r = call("POST", "/api/projects/5/shipments", T["farmer"], {
    "adoption_id": new_aid, "seq": 1, "plan_date": "2026-10-17",
    "pack_spec": "叶菜8斤箱", "cold_route": "烟台-杭州冷链", "weight_kg": 4})
check("编排序批发货计划", st == 201 and r["status"] == "planned", r)
sid = r["id"]
for st_ in ("picked", "packed"):
    st, r = call("POST", f"/api/projects/5/shipments/{sid}/advance", T["farmer"], {"status": st_})
    check(f"发货流转->{st_}", st == 200 and r["status"] == st_, r)
st, r = call("POST", f"/api/projects/5/shipments/{sid}/advance", T["logistics"],
             {"status": "shipped", "logistics_company": "顺丰冷运", "tracking_no": "SFVEG001",
              "bad_fruit_rate": 1.5})
check("物流发车填运单与坏果率", st == 200 and r["tracking_no"] == "SFVEG001", r)
st, r = call("POST", f"/api/projects/5/shipments/{sid}/advance", T["logistics"], {"status": "planned"})
check("非法状态跳转被拒", st == 400, r)
st, r = call("POST", f"/api/projects/5/shipments/{sid}/advance", T["logistics"], {"status": "delivered"})
check("物流确认送达", st == 200 and r["status"] == "delivered", r)
st, ad = call("GET", f"/api/adoptions/{new_aid}", T["adopter2"])
check("全部送达后订单自动完成", ad["status"] == "completed", ad)

# ==================== 成熟度分批发货（完整链路） ====================
# 吴琳(adopter2) 新认养稻田 1 份（保底30kg），农场按成熟度拆成 2 轮，逐轮写明重量与口感差异
st, r = call("POST", "/api/adoptions", T["adopter2"], {
    "project_id": 4, "shares": 1, "recipient_name": "吴琳", "phone": "13900000008",
    "address": "杭州市西湖区文三路478号"})
check("成熟度分批：吴琳新认养稻田1份", st == 201 and r["cert_no"].startswith("CERT-RICE-2026-A"), r)
rice_aid = r["adoption"]["id"]

def my_order(aid):
    st, mine = call("GET", "/api/my/adoptions", T["adopter2"])
    return next(a for a in mine if a["id"] == aid)

o0 = my_order(rice_aid)
check("权益核对含已送达/在途/待发/余额字段",
      all(k in o0["rights_fulfillment"] for k in
          ["delivered_kg", "in_transit_kg", "pending_kg", "remaining_kg"]), o0["rights_fulfillment"])
check("期初保底30kg、余额30、暂无分批方案",
      o0["rights_fulfillment"]["guarantee_kg"] == 30 and
      o0["rights_fulfillment"]["remaining_kg"] == 30 and o0["delivery_plans"] == [], o0["rights_fulfillment"])

ROUNDS = [
    {"seq": 1, "est_weight_kg": 20, "taste_diff": "首批稻穗金黄饱满、新米清香有嚼劲",
     "maturity_note": "主田块先熟，10月10日开镰", "planned_date": "2026-10-10",
     "pack_spec": "稻花香2号 10kg袋装", "cold_route": "烟台-杭州普通快运"},
    {"seq": 2, "est_weight_kg": 10, "taste_diff": "边角后熟田，米质略糯、适合煮粥",
     "maturity_note": "边角低洼成熟晚4天", "planned_date": "2026-10-14",
     "pack_spec": "稻花香2号 5kg袋装×2", "cold_route": "烟台-杭州普通快运"},
]
# 认养人不能编制方案
st, _ = call("POST", "/api/projects/4/delivery-plans", T["adopter2"],
             {"adoption_id": rice_aid, "rounds": ROUNDS})
check("认养人不能编制分批方案(403)", st == 403)
# 校验：至少2轮、必须写口感差异
st, r = call("POST", "/api/projects/4/delivery-plans", T["farmer"],
             {"adoption_id": rice_aid, "rounds": [ROUNDS[0]]})
check("单轮方案被拒(至少2轮)", st == 400, r)
bad_rounds = [dict(ROUNDS[0]), dict(ROUNDS[1], taste_diff="")]
st, r = call("POST", "/api/projects/4/delivery-plans", T["farmer"],
             {"adoption_id": rice_aid, "rounds": bad_rounds})
check("缺少口感差异被拒", st == 400 and "口感" in r.get("error", ""), r)

st, r = call("POST", "/api/projects/4/delivery-plans", T["farmer"],
             {"adoption_id": rice_aid, "reason": "稻田边角低洼成熟晚4天，拆两轮收割", "rounds": ROUNDS})
check("农场编制2轮成熟度分批方案", st == 201 and r["round_count"] == 2 and r["total_weight_kg"] == 30, r)
plan_id = r["plan"]["id"]
check("方案初态=draft，轮次含预计重量与口感",
      r["plan"]["status"] == "draft" and
      r["plan"]["rounds"][0]["est_weight_kg"] == 20 and "清香" in r["plan"]["rounds"][0]["taste_diff"], r["plan"])
check("编制即生成期初保底权益台账",
      any(x["kind"] == "opening" and x["balance_kg"] == 30 for x in r["plan"]["rights_records"]),
      r["plan"]["rights_records"])

# 顺序与角色：采摘工/包装员不能提交，draft 不能被包装组直接确认/被认养人确认
st, _ = call("POST", f"/api/delivery-plans/{plan_id}/submit", T["picker"], {})
check("采摘工不能提交采摘计划(403)", st == 403)
st, _ = call("POST", f"/api/delivery-plans/{plan_id}/submit", T["packer"], {})
check("包装员不能提交采摘计划(403)", st == 403)
st, r = call("POST", f"/api/delivery-plans/{plan_id}/pack-confirm", T["packer"], {})
check("未提交不能包装确认(409)", st == 409, r)
st, _ = call("POST", f"/api/delivery-plans/{plan_id}/confirm", T["adopter2"], {})
check("包装组未确认前认养人不能确认(409)", st == 409)

# 农场提交 → 采摘计划同步包装组
st, r = call("POST", f"/api/delivery-plans/{plan_id}/submit", T["farmer"], {})
check("农场提交，采摘计划同步包装组", st == 200 and r["status"] == "submitted", r)
# 包装组确认采摘计划
st, r = call("POST", f"/api/delivery-plans/{plan_id}/pack-confirm", T["packer"], {})
check("包装组确认采摘计划", st == 200 and r["status"] == "pack_confirmed", r)
# 非本人认养人不能确认
st, _ = call("POST", f"/api/delivery-plans/{plan_id}/confirm", T["adopter"], {})
check("非本单认养人不能确认(403)", st == 403)
# 认养人确认 → 物流计划 / 采摘任务 / 权益核算同步
st, r = call("POST", f"/api/delivery-plans/{plan_id}/confirm", T["adopter2"], {"note": "同意分两轮"})
check("认养人确认后同步2运单+2采摘任务",
      st == 200 and r["round_count"] == 2 and
      all(x["shipment_id"] and x["harvest_task_id"] for x in r["plan"]["rounds"]), r)
check("确认后生成两轮排产权益流水",
      sum(1 for x in r["plan"]["rights_records"] if x["kind"] == "allocation") == 2,
      r["plan"]["rights_records"])

o1 = my_order(rice_aid)
rs1 = next(p for p in o1["delivery_plans"] if p["id"] == plan_id)["rights_summary"]
check("确认后：待发30kg、在途0、已送0、余额30",
      rs1["pending_kg"] == 30 and rs1["in_transit_kg"] == 0 and
      rs1["delivered_kg"] == 0 and rs1["remaining_kg"] == 30, rs1)

def plan_detail(tok= None):
    tok = tok or T["farmer"]
    st, d = call("GET", f"/api/delivery-plans/{plan_id}", tok)
    assert st == 200, d
    return d

d = plan_detail()
r1, r2 = sorted(d["rounds"], key=lambda x: x["seq"])
# 农场据轮次安排下一次采摘工人
st, r = call("POST", f"/api/delivery-plans/{plan_id}/rounds/{r1['id']}/picker", T["farmer"],
             {"picker_id": 4})
check("为第1轮安排采摘工赵秀兰", st == 200 and r["harvest_task"]["picker_id"] == 4, r)
st, r = call("POST", f"/api/delivery-plans/{plan_id}/rounds/{r2['id']}/picker", T["farmer"],
             {"picker_id": 6})
check("指派非采摘工账号被拒(400)", st == 400, r)
st, _ = call("POST", f"/api/delivery-plans/{plan_id}/rounds/{r2['id']}/picker", T["adopter2"],
             {"picker_id": 4})
check("认养人不能安排采摘工(403)", st == 403)

def fulfill_round(rnd, actual):
    # 采摘工回填 → 运单 picked/packed/shipped/delivered
    call("POST", f"/api/projects/4/harvests/{rnd['harvest_task_id']}/complete", T["picker"],
         {"actual_kg": actual, "bad_rate_pct": 1.0})
    sid = rnd["shipment_id"]
    for stt in ("picked", "packed"):
        call("POST", f"/api/projects/4/shipments/{sid}/advance", T["farmer"], {"status": stt})
    call("POST", f"/api/projects/4/shipments/{sid}/advance", T["logistics"],
         {"status": "shipped", "logistics_company": "顺丰快运", "tracking_no": f"SFRICE-{rnd['seq']}"})
    st, rr = call("POST", f"/api/projects/4/shipments/{sid}/advance", T["logistics"],
                  {"status": "delivered"})
    return st, rr

# 第1轮 20kg 发车→送达
d2 = plan_detail()
rd1 = next(x for x in d2["rounds"] if x["seq"] == 1)
st, _ = fulfill_round(rd1, 20)
check("第1轮运单送达", st == 200)
d3 = plan_detail(T["adopter2"])
rs_after1 = d3["rights_summary"]
kinds_after1 = [x["kind"] for x in d3["rights_records"]]
check("第1轮后：已送20、待发10、权益余额10",
      rs_after1["delivered_kg"] == 20 and rs_after1["pending_kg"] == 10 and
      rs_after1["remaining_kg"] == 10, rs_after1)
check("发车/送达写入权益流水", "dispatch" in kinds_after1 and "delivery" in kinds_after1, kinds_after1)
# 还有一轮未送达，归档被拦截
st, r = call("POST", f"/api/delivery-plans/{plan_id}/archive", T["farmer"], {})
check("未全部送达不能归档(409)", st == 409 and "未送达" in r.get("error", ""), r)

# 第2轮 10kg 安排工人、采摘、送达 → 余额0
d4 = plan_detail()
rd2 = next(x for x in d4["rounds"] if x["seq"] == 2)
call("POST", f"/api/delivery-plans/{plan_id}/rounds/{rd2['id']}/picker", T["farmer"], {"picker_id": 4})
st, _ = fulfill_round(rd2, 10)
check("第2轮运单送达", st == 200)
d5 = plan_detail(T["adopter2"])
check("两轮全部送达后权益余额为0", d5["rights_summary"]["remaining_kg"] == 0
      and d5["rights_summary"]["delivered_kg"] == 30, d5["rights_summary"])
# 权益归档
st, r = call("POST", f"/api/delivery-plans/{plan_id}/archive", T["farmer"],
             {"archive_note": "两轮新米全部送达，权益兑现完毕"})
check("全部送达后权益归档", st == 200 and r["status"] == "archived", r)
d6 = plan_detail(T["adopter2"])
check("归档后台账含 archive 流水且标记已归档",
      any(x["kind"] == "archive" and x["archived"] for x in d6["rights_records"]), d6["status"])

# 种子已归档方案（陈曦蓝莓订单4）在我的认养中可见
st, mine_cx = call("GET", "/api/my/adoptions", T["adopter"])
berry = next(a for a in mine_cx if a["id"] == 4)
check("种子蓝莓方案已归档并对认养人可见",
      any(p["status"] == "archived" for p in berry["delivery_plans"]), berry.get("delivery_plans"))
# 种子吴琳苹果方案1（pack_confirmed）在吴琳“我的认养”中等待确认
st, mine_wl = call("GET", "/api/my/adoptions", T["adopter2"])
apple3 = next(a for a in mine_wl if a["id"] == 3)
check("种子苹果3轮方案待吴琳确认，逐轮展示重量与口感差异",
      any(p["status"] == "pack_confirmed" and len(p["rounds"]) == 3 and
          all(x["taste_diff"] for x in p["rounds"]) for p in apple3["delivery_plans"]),
      apple3.get("delivery_plans"))


# ---- 坏果投诉：多角色处理流水串到同一工单 ----
st, r = call("POST", "/api/incidents", T["adopter2"], {
    "project_id": 5, "adoption_id": new_aid, "kind": "bad_fruit",
    "title": "蔬菜箱有两株压伤", "description": "开箱发现最下层两株油麦菜压伤"})
check("认养人提交坏果工单", st == 201, r)
iid_new = r["id"]
st, _ = call("POST", f"/api/incidents/{iid_new}/events", T["logistics"],
             {"action": "提供冷链温控记录", "note": "全程2-6℃，无脱冷"})
st, _ = call("POST", f"/api/incidents/{iid_new}/events", T["packer"],
             {"action": "包装复查", "note": "叶菜筐底部缓冲不足，加垫泡沫垫"})
st, _ = call("POST", f"/api/incidents/{iid_new}/events", T["farmer"],
             {"action": "同意补发", "note": "下批补发同价叶菜"})
st, _ = call("POST", f"/api/compensations/incident/{iid_new}", T["service"],
             {"amount": 8, "kind": "reship", "note": "补发油麦菜折值8元"})
st, detail = call("GET", f"/api/incidents/{iid_new}", T["service"])
roles = {e["actor_role"] for e in detail["events"]}
check("工单串联 认养人/物流/包装/农场/客服", {"adopter", "logistics", "packer", "farmer", "service"} <= roles, roles)
check("赔付后工单自动转 compensated", detail["incident"]["status"] == "compensated", detail["incident"])

# ---- 既有工单：坏果单 id=2 追加客服事件并解决 ----
st, r = call("POST", "/api/incidents/2/events", T["service"],
             {"action": "二次回访", "note": "客户满意", "status": "resolved"})
check("客服推进工单状态", st == 201 and r["action"] == "二次回访", r)
st, detail2 = call("GET", "/api/incidents/2", T["adopter"])
check("认养人可查看本批次完整处理流水", len(detail2["events"]) >= 6 and detail2["incident"]["status"] == "resolved")

# ---- 保险理赔：新建报案并推进到到账 ----
st, r = call("POST", "/api/insurance-claims", T["farmer"], {
    "project_id": 5, "policy_no": "PICC-WIND-2026-009", "kind": "大风",
    "loss_kg": 30, "amount": 600, "note": "大风掀膜损失"})
check("农场保险报案", st == 201 and r["status"] == "applied", r)
claim_id = r["id"]
for st_ in ("surveying", "approved", "settled"):
    st, r = call("POST", f"/api/insurance-claims/{claim_id}", T["service"], {"status": st_})
check("理赔推进到到账", st == 200 and r["status"] == "settled", r)

# ---- 回访（含下一季续认建议）----
st, r = call("POST", "/api/callbacks", T["adopter2"], {
    "adoption_id": new_aid, "satisfaction": 5, "quality_score": 5, "delivery_score": 4,
    "would_renew": True, "comment": "蔬菜新鲜、处理问题很快", "renew_suggestion": "希望增加水果胡萝卜"})
check("成熟度回访+续认建议", st == 201 and r["satisfaction"] == 5, r)
st, r = call("POST", "/api/callbacks", T["adopter2"], {
    "adoption_id": new_aid, "satisfaction": 9, "quality_score": 5, "delivery_score": 4})
check("回访评分越界被拒", st == 400, r)

# ---- 直播 / 参观 / 团购 ----
st, r = call("POST", "/api/projects/1/live", T["farmer"], {
    "title": "分拣车间夜场直播", "stream_url": "https://live.example.com/sort",
    "starts_at": "2026-10-21T19:00:00+08:00"})
check("农场创建直播", st == 201 and r["status"] == "scheduled", r)
live_id = r["id"]
st, r = call("POST", f"/api/live/{live_id}", T["farmer"], {"status": "live"})
check("开播状态流转", st == 200 and r["status"] == "live", r)
st, r = call("POST", "/api/visits", T["adopter"], {
    "plot_id": 2, "visit_date": "2026-11-01", "visitors": 2, "note": "蓝莓园参观"})
check("地块参观预约", st == 201 and r["status"] == "pending", r)
vid = r["id"]
st, r = call("POST", f"/api/visits/{vid}", T["farmer"], {"status": "approved", "note": "欢迎，安排午餐"})
check("农场审核参观预约", st == 200 and r["status"] == "approved", r)
st, r = call("POST", "/api/group-buys", T["adopter"], {
    "project_id": 4, "company_name": "明德教育集团", "contact": "刘老师",
    "phone": "13900000009", "shares": 80, "discount_pct": 12})
check("企业团购需求提交", st == 201 and r["status"] == "pending", r)
gbid = r["id"]
st, r = call("POST", f"/api/group-buys/{gbid}", T["farmer"], {"status": "confirmed"})
check("农场确认团购", st == 200 and r["status"] == "confirmed", r)

# ---- 公益认养下单（代捐）----
st, r = call("POST", "/api/adoptions", T["adopter"], {
    "project_id": 4, "shares": 2, "recipient_name": "西山敬老院", "phone": "13900000010",
    "address": "栖霞市西山镇敬老院", "is_charity": True, "donee": "西山敬老院",
    "message": "祝老人们秋收安康"})
check("公益代捐认养成功", st == 201 and r["cert_no"], r)
st, ch = call("GET", "/api/charity")
check("公益榜新增受益方记录", any(x["donee"] == "西山敬老院" for x in ch))

# ---- 地块完整账 ----
st, led = call("GET", "/api/plots/1/ledger", T["farmer"])
check("地块台账聚合完整链路",
      all(k in led for k in ["costs", "logs", "disasters", "maturity_checks",
                             "harvest_tasks", "sorting", "shipments", "incidents",
                             "compensations", "callbacks"]), led.keys())
check("地块成本台账>0", led["total_cost"] > 0, led["total_cost"])
check("地块台账含冰雹灾害", any(d.get("disaster_kind") == "冰雹" for d in led["disasters"]))
check("地块台账含赔付与回访", len(led["compensations"]) >= 2 and len(led["callbacks"]) >= 1)
check("地块平均满意度计算", led["avg_satisfaction"] >= 4, led["avg_satisfaction"])

# ---- 农场看板 / 平台总览 ----
st, dash = call("GET", "/api/farm/dashboard", T["farmer"])
check("农场看板含项目/地块/工单/发货",
      all(k in dash for k in ["projects", "plots", "open_incidents", "active_shipments", "stats"]))
check("看板统计销售额", dash["stats"]["total_sales"] > 0, dash["stats"])
st, r = call("GET", "/api/admin/overview", T["adopter"])
check("非管理员禁止平台总览", st == 403)
st, ov = call("GET", "/api/admin/overview", T["admin"])
check("平台总览含分类项目与结算", "projects_by_kind" in ov and "recent_settlements" in ov, ov.keys())
check("平台总览含公益份额/保险到账/团购数",
      ov["stats"]["charity_shares"] >= 12 and ov["stats"]["insurance_settled"] >= 6000, ov["stats"])

# ---- 最终结算（蔬菜季，沉淀产量/成本/满意度/续认率）----
st, r = call("POST", "/api/projects/5/settle", T["farmer"],
             {"summary": "蔬菜季按时交付，坏果投诉快速闭环，客户满意，建议下季增水果胡萝卜。"})
check("生成最终交付结算", st == 200 and r["settlement"]["total_yield_kg"] == 96, r)
check("结算含赔付与满意度", r["settlement"]["total_compensation"] >= 8 and r["settlement"]["avg_satisfaction"] == 5,
      r["settlement"])
st, p5b = call("GET", "/api/projects/5")
check("结算后项目标记完成", p5b["project"]["status"] == "completed", p5b["project"]["status"])

# ---- 权限边界 ----
st, r = call("POST", "/api/projects", T["adopter"], {"plot_id": 1, "batch_no": "X", "kind": "apple",
                                                     "title": "x", "price_per_share": 1})
check("认养人不能发布项目", st == 403)
st, r = call("POST", "/api/projects/1/checks", T["logistics"], {"maturity_pct": 1})
check("非农场人员不能登记抽检(应403)", st == 403, r)

passed = sum(1 for _, c, _ in results if c)
print(f"\n==== {passed}/{len(results)} 通过 ====")
sys.exit(0 if passed == len(results) else 1)
