-- 成熟度分批发货：同一认养批次成熟不均时，平台把发货拆成多轮，
-- 每轮说明预计重量与口感差异；认养人确认后同步生成物流计划、更新权益核算，
-- 农场据此安排下一次采摘工人，采摘计划同步包装组确认；
-- 分批发货后认养人可见已发/待发/权益余额，全部送达后权益归档。

-- ---------- 1. 分轮发货方案（头 + 每轮明细） ----------
CREATE TABLE IF NOT EXISTS shipment_round_plans (
    id            BIGSERIAL PRIMARY KEY,
    project_id    BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    batch_no      TEXT NOT NULL,
    adoption_id   BIGINT NOT NULL REFERENCES adoptions(id) ON DELETE CASCADE,
    reason        TEXT NOT NULL DEFAULT '',        -- 成熟不均的原因说明
    status        TEXT NOT NULL DEFAULT 'proposed'
                    CHECK (status IN ('proposed','confirmed','rejected','archived')),
    created_by    BIGINT REFERENCES users(id),
    confirmed_by  BIGINT REFERENCES users(id),
    confirmed_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS shipment_round_items (
    id            BIGSERIAL PRIMARY KEY,
    plan_id       BIGINT NOT NULL REFERENCES shipment_round_plans(id) ON DELETE CASCADE,
    project_id    BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    adoption_id   BIGINT NOT NULL REFERENCES adoptions(id) ON DELETE CASCADE,
    seq           INT NOT NULL,                    -- 第几轮发货
    plan_date     DATE,
    est_weight_kg NUMERIC(10,2) NOT NULL DEFAULT 0,-- 该轮预计重量
    taste_diff    TEXT NOT NULL DEFAULT '',        -- 该轮口感差异说明
    pack_spec     TEXT NOT NULL DEFAULT '',
    cold_route    TEXT NOT NULL DEFAULT '',
    shipment_id   BIGINT REFERENCES shipments(id) ON DELETE SET NULL, -- 确认后生成/关联的物流运单
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (plan_id, seq)
);

ALTER TABLE shipments ADD COLUMN IF NOT EXISTS round_plan_id BIGINT REFERENCES shipment_round_plans(id) ON DELETE SET NULL;
ALTER TABLE shipments ADD COLUMN IF NOT EXISTS round_item_id BIGINT REFERENCES shipment_round_items(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_round_plans_adoption ON shipment_round_plans(adoption_id);
CREATE INDEX IF NOT EXISTS idx_round_items_plan ON shipment_round_items(plan_id);

-- ---------- 2. 采摘计划（农场据分轮方案安排下一次采摘工人 → 同步包装组确认） ----------
CREATE TABLE IF NOT EXISTS harvest_plans (
    id             BIGSERIAL PRIMARY KEY,
    project_id     BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    batch_no       TEXT NOT NULL,
    round_plan_id  BIGINT REFERENCES shipment_round_plans(id) ON DELETE SET NULL,
    round_seq      INT,                            -- 为第几轮发货备货
    picker_id      BIGINT REFERENCES users(id),    -- 安排的采摘工人/组长
    planned_date   DATE NOT NULL,
    target_kg      NUMERIC(10,2) NOT NULL DEFAULT 0,
    worker_note    TEXT NOT NULL DEFAULT '',       -- 用工安排（人数/时段/注意事项）
    fruit_note     TEXT NOT NULL DEFAULT '',       -- 采摘对象（晚熟内膛果、成熟度门槛等）
    status         TEXT NOT NULL DEFAULT 'planned'
                     CHECK (status IN ('planned','confirmed','done','canceled')),
    created_by     BIGINT REFERENCES users(id),
    packer_id      BIGINT REFERENCES users(id),    -- 确认计划的包装组
    packer_note    TEXT NOT NULL DEFAULT '',
    confirmed_at   TIMESTAMPTZ,
    harvest_task_id BIGINT REFERENCES harvest_tasks(id) ON DELETE SET NULL, -- 确认后生成的采摘任务
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_harvest_plans_project ON harvest_plans(project_id);
CREATE INDEX IF NOT EXISTS idx_harvest_plans_status ON harvest_plans(status);

-- ---------- 3. 权益归档与余额（每单一行，发货后滚动同步，全部送达后归档） ----------
CREATE TABLE IF NOT EXISTS rights_archives (
    id                   BIGSERIAL PRIMARY KEY,
    adoption_id          BIGINT NOT NULL UNIQUE REFERENCES adoptions(id) ON DELETE CASCADE,
    project_id           BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    batch_no             TEXT NOT NULL,
    guarantee_kg         NUMERIC(10,2) NOT NULL DEFAULT 0, -- 保底权益总量
    shipped_kg           NUMERIC(10,2) NOT NULL DEFAULT 0, -- 已发（已出库及送达）
    pending_kg           NUMERIC(10,2) NOT NULL DEFAULT 0, -- 待发（已确认方案中尚未出库）
    compensated_kg_equiv NUMERIC(10,2) NOT NULL DEFAULT 0, -- 赔付折算产量
    balance_kg           NUMERIC(10,2) NOT NULL DEFAULT 0, -- 权益余额（尚无发货/赔付覆盖的欠量）
    status               TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','archived')),
    summary              TEXT NOT NULL DEFAULT '',
    synced_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    archived_at          TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_rights_archives_project ON rights_archives(project_id);

-- ========================================================================
-- 演示数据：APPLE-2026-A 东坡3号园同一批次成熟不均（外围果先熟、内膛果晚熟约 10 天）
-- ========================================================================

-- 陈曦订单(id=1)：已与认养人确认的两轮方案（首批已送达、第二批待发）
INSERT INTO shipment_round_plans (id, project_id, batch_no, adoption_id, reason, status, created_by, confirmed_by, confirmed_at, created_at) VALUES
 (1,1,'APPLE-2026-A',1,
  '东坡3号园同一批次树势与光照不均：外围向阳果 10 月 18 日已达糖度 14% 以上先采先发；内膛晚熟果淀粉染色仍有 2-3 级，约晚 10 天达到最佳口感。为避免早采导致风味不足，平台将本订单拆为两轮发货。',
  'confirmed',2,3,'2026-10-16 15:20:00+08','2026-10-16 10:00:00+08')
ON CONFLICT (id) DO NOTHING;
SELECT setval('shipment_round_plans_id_seq', (SELECT MAX(id) FROM shipment_round_plans));

INSERT INTO shipment_round_items (id, plan_id, project_id, adoption_id, seq, plan_date, est_weight_kg, taste_diff, pack_spec, cold_route, shipment_id) VALUES
 (1,1,1,1,1,'2026-10-18',12.00,
  '外围向阳果：糖度 14.6%，脆甜多汁、着色面≥85%，到手即食，为当季最佳一批。',
  '精品大果2箱+标准中果1箱（共24斤）','烟台-济南分拨-北京朝阳（全程0-4℃冷链车 鲁F·D7721）',1),
 (2,1,1,1,2,'2026-10-23',8.00,
  '内膛晚熟果：糖度约 13.2%、酸度略高、果肉更紧实，建议常温后熟 3-5 天再食用，更耐储存；剩余缺口按冰雹赔付方案折算。',
  '标准中果2箱+小果1箱（共16斤，剩余按赔付方案处理）','烟台-杭州（冷链车 鲁F·D7721，按认养人改址更新）',2)
ON CONFLICT (id) DO NOTHING;
SELECT setval('shipment_round_items_id_seq', (SELECT MAX(id) FROM shipment_round_items));

UPDATE shipments SET round_plan_id=1, round_item_id=1 WHERE id=1;
UPDATE shipments SET round_plan_id=1, round_item_id=2,
       cold_route='烟台-杭州分拨（冷链车 鲁F·D7721，按认养人改址更新，加冰袋2个）'
WHERE id=2;

-- 陈曦订单的第二次采摘计划（为第2轮晚熟果备货）：农场安排 → 同步包装组 → 包装组已确认，并已生成采摘任务
INSERT INTO harvest_plans (id, project_id, batch_no, round_plan_id, round_seq, picker_id, planned_date, target_kg,
    worker_note, fruit_note, status, created_by, packer_id, packer_note, confirmed_at, harvest_task_id, created_at) VALUES
 (1,1,'APPLE-2026-A',1,2,4,'2026-10-21',1200.00,
  '安排采摘工 6 人，上午带果柄轻采、下午分拣入库，单筐不超过 30 斤防碰压。',
  '只采内膛晚熟果：糖度≥13%、淀粉染色 2-3 级，雹伤果转加工渠道不进入认养箱。',
  'confirmed',2,5,'已核对包装规格与箱材：中果 15 枚装隔板、小果 20 枚装网套已备齐，可按计划采摘。',
  '2026-10-17 09:30:00+08',2,'2026-10-16 16:00:00+08')
ON CONFLICT (id) DO NOTHING;
SELECT setval('harvest_plans_id_seq', (SELECT MAX(id) FROM harvest_plans));

-- 陈曦权益余额（首批已发 12kg、第二批待发 8kg、赔付折算约 20.03kg，保底 40kg，余额 0）
INSERT INTO rights_archives (id, adoption_id, project_id, batch_no, guarantee_kg, shipped_kg, pending_kg,
    compensated_kg_equiv, balance_kg, status, summary, synced_at) VALUES
 (1,1,1,'APPLE-2026-A',40.00,12.00,8.00,20.03,0.00,'active',
  '分两轮发货：第1轮 12kg 已送达，第2轮 8kg 待发；坏果补发与冰雹保险赔付折合约 20.03kg，保底权益已覆盖。',
  '2026-10-22 12:00:00+08')
ON CONFLICT (adoption_id) DO NOTHING;
SELECT setval('rights_archives_id_seq', (SELECT MAX(id) FROM rights_archives));

-- 吴琳订单(id=3，10 棵树保底 200kg)：平台新拟、待认养人确认的两轮方案
INSERT INTO shipment_round_plans (id, project_id, batch_no, adoption_id, reason, status, created_by, created_at) VALUES
 (2,1,'APPLE-2026-A',3,
  '同一认养批次 APPLE-2026-A 成熟不均：吴琳名下果树外围果已达采摘糖度，内膛果晚熟约 10 天。平台按成熟度拆为两轮，分别说明预计重量与口感差异，待认养人确认后排产。',
  'proposed',2,'2026-10-17 11:00:00+08')
ON CONFLICT (id) DO NOTHING;
SELECT setval('shipment_round_plans_id_seq', (SELECT MAX(id) FROM shipment_round_plans));

INSERT INTO shipment_round_items (id, plan_id, project_id, adoption_id, seq, plan_date, est_weight_kg, taste_diff, pack_spec, cold_route, shipment_id) VALUES
 (3,2,1,3,1,'2026-10-28',120.00,
  '外围向阳果：糖度约 14.5%，脆甜多汁、着色面≥85%，到手即食。',
  '精品大果 + 标准中果（约 240 斤）','烟台-杭州（全程 0-4℃ 冷链）',NULL),
 (4,2,1,3,2,'2026-11-06',80.00,
  '内膛晚熟果：糖度约 13.2%、酸度略高、果肉紧实，建议常温后熟 3-5 天，更耐储存。',
  '标准中果 + 实惠小果（约 160 斤）','烟台-杭州（全程 0-4℃ 冷链）',NULL)
ON CONFLICT (id) DO NOTHING;
SELECT setval('shipment_round_items_id_seq', (SELECT MAX(id) FROM shipment_round_items));

-- 为吴琳方案第 1 轮预先拟好的采摘计划，已同步包装组、待包装组确认
INSERT INTO harvest_plans (id, project_id, batch_no, round_plan_id, round_seq, picker_id, planned_date, target_kg,
    worker_note, fruit_note, status, created_by, created_at) VALUES
 (2,1,'APPLE-2026-A',2,1,4,'2026-10-27',600.00,
  '拟安排采摘工 4 人，上午采外围果、当日预冷，需认养人确认分轮方案后执行。',
  '只采外围向阳成熟果：糖度≥14%、着色面≥80%。',
  'planned',2,'2026-10-17 11:10:00+08')
ON CONFLICT (id) DO NOTHING;
SELECT setval('harvest_plans_id_seq', (SELECT MAX(id) FROM harvest_plans));
