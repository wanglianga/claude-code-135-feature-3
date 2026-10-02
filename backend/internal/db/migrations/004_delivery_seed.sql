-- 成熟度分批发货演示数据
-- 场景一（进行中）：吴琳 10 份苹果（订单3，保底 200kg），东坡园同一批次成熟不均——
--   阳面果已上糖、内膛果尚青，平台按成熟度拆成 3 轮发货并逐轮写明预计重量与口感差异；
--   农场已提交、采摘计划已同步包装组并经包装员王梅确认，现等待认养人确认。
INSERT INTO delivery_plans (id, project_id, adoption_id, batch_no, reason, status, created_by,
    submitted_at, packer_id, pack_confirmed_at) VALUES
 (1,1,3,'APPLE-2026-A',
    '东坡3号园同一批次成熟不均：树冠阳面果已摘袋上糖、着色全红，内膛与背阴果着色约六成、淀粉尚未完全转化。为保证每轮到家都在最佳赏味期，按成熟度拆成3轮采摘发货。',
    'pack_confirmed',2,'2026-10-02 09:30:00+08',5,'2026-10-02 10:15:00+08')
ON CONFLICT (id) DO NOTHING;

INSERT INTO delivery_plan_rounds (id, plan_id, project_id, batch_no, seq, est_weight_kg, taste_diff,
    maturity_note, planned_date, pack_spec, cold_route) VALUES
 (1,1,1,'APPLE-2026-A',1,90.00,
    '阳面头采果：糖度约14.5%，脆甜多汁、着色面≥90%、果香最浓',
    '淀粉染色1-2级，已达全熟，10月22日先采阳面树冠',
    '2026-10-22','精品大果80-85mm/12枚装','烟台-济南分拨-杭州（全程0-4℃）'),
 (2,1,1,'APPLE-2026-A',2,80.00,
    '中段过渡果：糖度约13.2%，脆爽带清新微酸、果径略小，适合喜欢爽口口感',
    '淀粉染色3级，约7-8成熟，再挂枝6天上糖后采摘',
    '2026-10-28','标准中果75-80mm/15枚装','烟台-杭州（冷链车 鲁F·D7721）'),
 (3,1,1,'APPLE-2026-A',3,30.00,
    '内膛后熟果：糖度约14%，下树后常温存放3天转为粉糯香甜，适合老人儿童',
    '淀粉染色4-5级，末轮清园采摘，建议收到后催熟3天再食用',
    '2026-11-03','实惠中果+小果混装','烟台-杭州（冷链车 鲁F·D7721）')
ON CONFLICT (id) DO NOTHING;

-- 场景二（已归档）：陈曦蓝莓订单4（项目2，保底3kg）分一轮发出并签收，权益台账完整归档
INSERT INTO delivery_plans (id, project_id, adoption_id, batch_no, reason, status, created_by,
    submitted_at, packer_id, pack_confirmed_at, adopter_confirmed_at, archive_note, archived_at) VALUES
 (2,2,4,'BERRY-2026-A','蓝莓全园成熟基本一致，按1轮鲜果冷链发出。','archived',2,
    '2026-06-24 09:00:00+08',5,'2026-06-24 10:00:00+08','2026-06-24 12:00:00+08',
    '保底3kg鲜果一轮送达签收，权益全部兑现，台账归档。','2026-06-27 10:30:00+08')
ON CONFLICT (id) DO NOTHING;

INSERT INTO delivery_plan_rounds (id, plan_id, project_id, batch_no, seq, est_weight_kg, taste_diff,
    maturity_note, planned_date, pack_spec, cold_route, shipment_id) VALUES
 (4,2,2,'BERRY-2026-A',1,3.00,
    '优瑞卡全熟果：糖度11.8%，果粉完整、酸甜浓郁',
    '成熟度98%，采后2小时内预冷',
    '2026-06-25','优瑞卡鲜果6斤礼盒装','烟台-北京（生鲜专机+冷链）',4)
ON CONFLICT (id) DO NOTHING;

-- 权益台账：期初保底 → 排产占位 → 发车 → 送达兑现 → 归档
INSERT INTO rights_records (id, project_id, batch_no, adoption_id, plan_id, round_id, kind,
    change_kg, amount, balance_kg, note, archived, operator_id, created_at) VALUES
 -- 吴琳苹果（待确认，仅有期初保底）
 (1,1,'APPLE-2026-A',3,1,NULL,'opening',200.00,0,200.00,
    '期初保底权益：10份 × 20kg = 200kg（成熟不均，分3轮发货）',FALSE,2,'2026-07-10 10:05:00+08'),
 -- 陈曦蓝莓（已归档完整流水）
 (2,2,'BERRY-2026-A',4,2,NULL,'opening',3.00,0,3.00,
    '期初保底权益：1份 × 3kg = 3kg',FALSE,3,'2026-05-20 11:00:00+08'),
 (3,2,'BERRY-2026-A',4,2,4,'allocation',0,0,3.00,
    '第1轮排产 3kg，物流计划与采摘计划已同步',FALSE,2,'2026-06-24 12:05:00+08'),
 (4,2,'BERRY-2026-A',4,2,4,'dispatch',0,0,3.00,
    '第1轮冷链发车（SFFRESH2026062501），3kg 在途',FALSE,6,'2026-06-25 09:00:00+08'),
 (5,2,'BERRY-2026-A',4,2,4,'delivery',-3.00,0,0.00,
    '第1轮送达签收，兑现 3kg',FALSE,6,'2026-06-26 10:00:00+08'),
 (6,2,'BERRY-2026-A',4,2,NULL,'archive',0,0,0.00,
    '保底权益全部兑现，权益归档',TRUE,7,'2026-06-27 10:30:00+08')
ON CONFLICT (id) DO NOTHING;

SELECT setval('delivery_plans_id_seq', (SELECT MAX(id) FROM delivery_plans));
SELECT setval('delivery_plan_rounds_id_seq', (SELECT MAX(id) FROM delivery_plan_rounds));
SELECT setval('rights_records_id_seq', (SELECT MAX(id) FROM rights_records));
