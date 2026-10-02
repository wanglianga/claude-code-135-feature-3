-- 成熟度分批发货
-- 背景：同一认养批次（project_id + batch_no）果实成熟不均时，平台/农场把发货拆成
-- 多轮（delivery_plan_rounds），逐轮写明“预计重量 + 口感差异 + 成熟度说明”。
-- 协作链：农场/平台编制方案(draft) → 提交后采摘计划同步包装组(submitted)
--   → 包装组确认采摘计划(pack_confirmed) → 认养人确认(adopter_confirmed)
--   → 同步生成物流计划(shipments)、采摘任务(harvest_tasks)、权益台账(rights_records)
--   → 逐轮采摘/发货/送达，权益余额同步认养人，农场据轮次安排下一次采摘工人
--   → 全部送达后权益归档(archived)。

-- 成熟度分批发货方案（按认养订单编制，挂在同一认养批次上）
CREATE TABLE IF NOT EXISTS delivery_plans (
    id                   BIGSERIAL PRIMARY KEY,
    project_id           BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    adoption_id          BIGINT NOT NULL REFERENCES adoptions(id) ON DELETE CASCADE,
    batch_no             TEXT NOT NULL,
    reason               TEXT NOT NULL DEFAULT '',        -- 成熟不均情况说明
    status               TEXT NOT NULL DEFAULT 'draft'
                           CHECK (status IN ('draft','submitted','pack_confirmed',
                                             'adopter_confirmed','archived','cancelled')),
    created_by           BIGINT REFERENCES users(id),
    submitted_at         TIMESTAMPTZ,                     -- 采摘计划同步包装组时间
    packer_id            BIGINT REFERENCES users(id),
    pack_confirmed_at    TIMESTAMPTZ,                     -- 包装组确认采摘计划时间
    adopter_confirmed_at TIMESTAMPTZ,                     -- 认养人确认时间（物流/权益同步）
    archive_note         TEXT NOT NULL DEFAULT '',
    archived_at          TIMESTAMPTZ,                     -- 权益归档时间
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 每一轮发货：预计重量、口感差异（糖度/脆面/香气）、成熟度说明
CREATE TABLE IF NOT EXISTS delivery_plan_rounds (
    id              BIGSERIAL PRIMARY KEY,
    plan_id         BIGINT NOT NULL REFERENCES delivery_plans(id) ON DELETE CASCADE,
    project_id      BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    batch_no        TEXT NOT NULL,
    seq             INT NOT NULL,                        -- 第几轮
    est_weight_kg   NUMERIC(10,2) NOT NULL DEFAULT 0,    -- 本轮预计重量
    taste_diff      TEXT NOT NULL DEFAULT '',            -- 口感差异说明
    maturity_note   TEXT NOT NULL DEFAULT '',            -- 成熟度说明
    planned_date    DATE,
    pack_spec       TEXT NOT NULL DEFAULT '',
    cold_route      TEXT NOT NULL DEFAULT '',
    shipment_id     BIGINT REFERENCES shipments(id) ON DELETE SET NULL,       -- 确认后生成的物流计划
    harvest_task_id BIGINT REFERENCES harvest_tasks(id) ON DELETE SET NULL,   -- 确认后生成的采摘任务
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (plan_id, seq)
);

-- 认养人权益台账（可归档）：期初保底 / 排产占位 / 发车 / 送达 / 赔付折算 / 归档
CREATE TABLE IF NOT EXISTS rights_records (
    id          BIGSERIAL PRIMARY KEY,
    project_id  BIGINT REFERENCES projects(id) ON DELETE CASCADE,
    batch_no    TEXT NOT NULL DEFAULT '',
    adoption_id BIGINT NOT NULL REFERENCES adoptions(id) ON DELETE CASCADE,
    plan_id     BIGINT REFERENCES delivery_plans(id) ON DELETE SET NULL,
    round_id    BIGINT REFERENCES delivery_plan_rounds(id) ON DELETE SET NULL,
    kind        TEXT NOT NULL CHECK (kind IN ('opening','allocation','dispatch','delivery','compensation','archive')),
    change_kg   NUMERIC(10,2) NOT NULL DEFAULT 0,   -- 正=新增待兑现权益，负=兑现扣减，排产占位不影响余额
    amount      NUMERIC(12,2) NOT NULL DEFAULT 0,   -- 涉及金额（赔付时）
    balance_kg  NUMERIC(10,2),                      -- 该笔后权益余额（尚未兑现的保底产量）
    note        TEXT NOT NULL DEFAULT '',
    archived    BOOLEAN NOT NULL DEFAULT FALSE,
    operator_id BIGINT REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 采摘任务反向关联到所属分轮，方便农场“据此安排下一次采摘工人”
ALTER TABLE harvest_tasks
    ADD COLUMN IF NOT EXISTS plan_round_id BIGINT REFERENCES delivery_plan_rounds(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_delivery_plans_project ON delivery_plans(project_id);
CREATE INDEX IF NOT EXISTS idx_delivery_plans_adoption ON delivery_plans(adoption_id);
CREATE INDEX IF NOT EXISTS idx_plan_rounds_plan ON delivery_plan_rounds(plan_id);
CREATE INDEX IF NOT EXISTS idx_rights_adoption ON rights_records(adoption_id);
CREATE INDEX IF NOT EXISTS idx_rights_project ON rights_records(project_id, batch_no);
