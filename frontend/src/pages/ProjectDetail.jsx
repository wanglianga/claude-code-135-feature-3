import React, { useCallback, useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import {
  get, post, KIND_EMOJI, INCIDENT_LABEL, INCIDENT_COLOR,
  fmtMoney, fmtKg, fmtDay, fmtDate, PLAN_STATUS_LABEL,
} from '../api'
import { Tag, KindTag, ShipFlow, Modal, Empty, Stat, useToast } from '../components/ui'
import { useAuth } from '../auth'

const LOG_LABEL = { farming: '农事', fertilizer: '施肥', pesticide: '用药', disaster: '灾害', photo: '照片', other: '其他' }
const LOG_CLS = { farming: 'green', fertilizer: 'amber', pesticide: 'purple', disaster: 'red', photo: 'blue', other: 'gray' }
const PROJECT_STATUS = { ongoing: '认养中', harvesting: '采摘配送中', completed: '已结季' }

export default function ProjectDetail() {
  const { id } = useParams()
  const { user } = useAuth()
  const [d, setD] = useState(null)
  const [tab, setTab] = useState('logs')
  const [action, setAction] = useState(null)

  const load = useCallback(() => get(`/projects/${id}`).then(setD), [id])
  useEffect(() => { load() }, [load])

  if (!d) return <Empty text="加载中…" />
  const p = d.project
  const ownFarm = user && user.farm_id === p.farm_id
  const canOperate = user && (user.role === 'admin' || ownFarm)
  const isLogistics = user && user.role === 'logistics'

  const TABS = [
    ['logs', `农事日志 (${d.logs.length})`],
    ['checks', `成熟度抽检 (${d.maturity_checks.length})`],
    ['harvest', `采摘·分拣·发货 (${d.shipments.length})`],
    ['plans', `成熟度分批 (${(d.delivery_plans || []).length})`],
    ['incidents', `异常工单 (${d.incidents.length})`],
    ['delivery', '最终交付'],
  ]

  return (
    <>
      <div className="card">
        <div className="row" style={{ alignItems: 'flex-start' }}>
          <div className="proj-emoji">{KIND_EMOJI[p.kind]}</div>
          <div style={{ flex: 1 }}>
            <div className="row">
              <KindTag kind={p.kind} />
              <Tag cls={p.status === 'completed' ? 'gray' : 'amber'}>{PROJECT_STATUS[p.status]}</Tag>
              {p.is_charity && <Tag cls="red">❤️ 公益认养</Tag>}
              {p.insurance_policy && <Tag cls="teal">🛡 {p.insurance_policy}</Tag>}
            </div>
            <h1 className="mt8">{p.title}</h1>
            <div className="muted small">
              {p.farm_name}（{p.farm_location}） · {p.plot_name} {p.area_mu} 亩 · 批次 {p.batch_no}
            </div>
          </div>
          <div style={{ textAlign: 'right' }}>
            <div className="price" style={{ fontSize: 22 }}>{fmtMoney(p.price_per_share)}</div>
            <div className="small muted">每{p.unit_name} · 保底 {p.guarantee_kg}kg</div>
            <div className="small muted mt8">已认养 {p.sold_shares}/{p.total_shares} {p.unit_name}</div>
          </div>
        </div>
        <p className="mt16">{p.description}</p>
        <div className="grid grid-3 mt8">
          <div><h3>📋 采摘标准</h3><div className="small muted">{p.harvest_standard}</div></div>
          <div><h3>🚚 配送范围</h3><div className="small muted">{p.delivery_range}</div></div>
          <div><h3>🎁 认养人权益</h3><div className="small muted">{p.rights_text}</div></div>
        </div>
      </div>

      <div className="grid grid-4">
        <Stat label="预计成熟日期" value={fmtDay(p.est_mature_date)} />
        <Stat label="累计实际采摘" value={fmtKg(d.sum_harvested)} cls="amber" />
        <Stat label="累计已发货" value={fmtKg(d.sum_shipped)} cls="blue" />
        <Stat label="在途/待处理工单" value={d.incidents.filter(i => i.status !== 'resolved' && i.status !== 'compensated').length} cls="red" />
      </div>

      {canOperate && (
        <div className="card">
          <div className="section-title">农场操作</div>
          <div className="row">
            <button className="sm" onClick={() => setAction('log')}>📷 记农事/灾害</button>
            <button className="sm" onClick={() => setAction('check')}>🔬 成熟度抽检</button>
            <button className="sm" onClick={() => setAction('harvest')}>🧺 安排采摘</button>
            <button className="sm" onClick={() => setAction('sorting')}>📦 登记分拣</button>
            <button className="sm" onClick={() => setAction('shipment')}>🚚 编排发货</button>
            <button className="sm" onClick={() => setAction({ type: 'plan', adoptions: d.adoption_list || [] })}>🍎 成熟度分批</button>
            <button className="sm" onClick={() => setAction('cost')}>💰 地块记账</button>
            <button className="sm ghost" onClick={() => setAction('live')}>🎥 开直播</button>
            {p.status !== 'completed' && <button className="sm amber" onClick={() => setAction('settle')}>✅ 季末结算</button>}
          </div>
        </div>
      )}

      <div className="card">
        <div className="tabs">
          {TABS.map(([k, label]) => (
            <button key={k} className={tab === k ? 'active' : ''} onClick={() => setTab(k)}>{label}</button>
          ))}
        </div>

        {tab === 'logs' && (
          <div className="timeline">
            {d.logs.map((l) => (
              <div key={l.id} className={'tl-item' + (l.log_type === 'disaster' ? ' disaster' : '')}>
                <div className="row" style={{ justifyContent: 'space-between' }}>
                  <strong>{l.title}</strong>
                  <span>
                    <Tag cls={LOG_CLS[l.log_type]}>{LOG_LABEL[l.log_type]}</Tag>
                    {l.disaster_kind && <Tag cls="red">{l.disaster_kind} · 影响约 {l.yield_impact_pct}%</Tag>}
                  </span>
                </div>
                <div className="small">{l.content}</div>
                {l.photos?.length > 0 && (
                  <div className="small muted mt8">📷 {l.photos.map((x) => x.caption).join('、')}</div>
                )}
                <div className="tl-time">{l.author_name} · {fmtDate(l.created_at)}</div>
              </div>
            ))}
          </div>
        )}

        {tab === 'checks' && (
          <>
            <table>
              <thead><tr><th>抽检时间</th><th>糖度</th><th>样本</th><th>成熟度</th><th>预估产量</th><th>坏果率</th><th>分级</th><th>备注</th></tr></thead>
              <tbody>
                {d.maturity_checks.map((c) => (
                  <tr key={c.id}>
                    <td>{fmtDate(c.checked_at)}</td>
                    <td>{c.brix ?? '—'}</td>
                    <td>{c.sample_count}</td>
                    <td><Tag cls={c.maturity_pct >= 90 ? 'green' : 'amber'}>{c.maturity_pct}%</Tag></td>
                    <td>{fmtKg(c.est_yield_kg)}</td>
                    <td className={c.bad_rate_pct > 4 ? '' : ''} style={{ color: c.bad_rate_pct > 4 ? 'var(--red)' : undefined }}>
                      {c.bad_rate_pct}%
                    </td>
                    <td className="small">{Object.entries(c.grade_dist || {}).map(([k, v]) => `${k} ${v}%`).join(' / ')}</td>
                    <td className="small muted">{c.notes}</td>
                  </tr>
                ))}
              </tbody>
            </table>
            {d.maturity_checks.length === 0 && <Empty />}
          </>
        )}

        {tab === 'harvest' && (
          <div>
            <h3>🧺 采摘任务</h3>
            <table>
              <thead><tr><th>计划日期</th><th>优先级</th><th>采摘工</th><th>目标</th><th>实际</th><th>坏果率</th><th>状态</th>
                  {user && (user.role === 'picker' || canOperate) && <th>操作</th>}</tr></thead>
              <tbody>
                {d.harvest_tasks.map((h) => (
                  <tr key={h.id}>
                    <td>{fmtDay(h.planned_date)}</td>
                    <td>{h.priority}</td>
                    <td>{h.picker_name || '未分配'}</td>
                    <td>{fmtKg(h.target_kg)}</td>
                    <td>{h.actual_kg != null ? fmtKg(h.actual_kg) : '—'}</td>
                    <td>{h.bad_rate_pct != null ? h.bad_rate_pct + '%' : '—'}</td>
                    <td><Tag cls={h.status === 'done' ? 'gray' : 'amber'}>{h.status === 'done' ? '已完成' : '待采摘'}</Tag></td>
                    <td>
                      {h.status !== 'done' && (user.role === 'picker' || canOperate) && (
                        <button className="sm ghost" onClick={() => setAction({ type: 'harvest_done', task: h })}>回填</button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {d.harvest_tasks.length === 0 && <Empty text="暂无采摘任务" />}

            <h3 className="mt16">📦 分拣记录（包装规格）</h3>
            <table>
              <thead><tr><th>规格</th><th>重量</th><th>箱数</th><th>包装员</th><th>时间</th></tr></thead>
              <tbody>
                {d.sorting.map((s) => (
                  <tr key={s.id}><td>{s.spec}</td><td>{fmtKg(s.weight_kg)}</td><td>{s.box_count}</td>
                    <td>{s.packer_name}</td><td>{fmtDate(s.created_at)}</td></tr>
                ))}
              </tbody>
            </table>
            {d.sorting.length === 0 && <Empty text="暂无分拣记录" />}

            <h3 className="mt16">🚚 分批发货与冷链</h3>
            <table>
              <thead><tr><th>批次</th><th>计划</th><th>规格</th><th>重量</th><th>冷链路线</th><th>状态</th><th>运单号</th><th>坏果率</th><th>收货人</th>
                {(canOperate || isLogistics) && <th>操作</th>}</tr></thead>
              <tbody>
                {d.shipments.map((s) => (
                  <tr key={s.id}>
                    <td>第 {s.seq} 批</td>
                    <td>{fmtDay(s.plan_date)}</td>
                    <td className="small">{s.pack_spec}</td>
                    <td>{s.weight_kg != null ? fmtKg(s.weight_kg) : '—'}</td>
                    <td className="small">{s.cold_route || '—'}</td>
                    <td><ShipFlow status={s.status} /></td>
                    <td className="small">{s.tracking_no || '—'}</td>
                    <td style={{ color: s.bad_fruit_rate > 4 ? 'var(--red)' : undefined }}>{s.status === 'planned' ? '—' : s.bad_fruit_rate + '%'}</td>
                    <td className="small">{s.recipient_name || '—'}</td>
                    <td>
                      {(canOperate || isLogistics) && s.status !== 'delivered' && (
                        <button className="sm ghost" onClick={() => setAction({ type: 'ship_advance', ship: s })}>推进</button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {d.shipments.length === 0 && <Empty text="暂无发货计划" />}
          </div>
        )}

        {tab === 'plans' && (
          <DeliveryPlansTab d={d} canOperate={canOperate} onReload={load} />
        )}

        {tab === 'incidents' && (
          <div className="grid grid-2">
            {d.incidents.map((i) => (
              <Link key={i.id} to={`/incidents?focus=${i.id}`} className="card" style={{ color: 'inherit', display: 'block' }}>
                <div className="row" style={{ justifyContent: 'space-between' }}>
                  <Tag cls={i.status === 'compensated' ? 'green' : i.status === 'resolved' ? 'gray' : 'red'}>
                    {i.status === 'open' ? '待处理' : i.status === 'processing' ? '处理中' : i.status === 'resolved' ? '已解决' : '已赔付'}
                  </Tag>
                  <span className="tag" style={{ background: '#fef2f2', color: INCIDENT_COLOR[i.kind], borderColor: '#fecaca' }}>
                    {INCIDENT_LABEL[i.kind]}
                  </span>
                </div>
                <h3 className="mt8">{i.title}</h3>
                <div className="small muted">{i.description}</div>
                <div className="tl-time mt8">{i.reporter_name} · {fmtDate(i.created_at)}</div>
              </Link>
            ))}
            {d.incidents.length === 0 && <Empty text="本批次无异常" />}
          </div>
        )}

        {tab === 'delivery' && <DeliveryTab d={d} />}
      </div>

      {action && <ActionModal kind={action} projectId={p.id} onClose={() => setAction(null)} onDone={load} />}
    </>
  )
}

function DeliveryTab({ d }) {
  const st = d.settlement
  const claims = d.insurance_claims || []
  return (
    <>
      {st ? (
        <div className="card" style={{ background: '#fafdf7' }}>
          <div className="section-title">📜 最终交付记录</div>
          <div className="grid grid-4">
            <Stat label="实际总产量" value={fmtKg(st.total_yield_kg)} />
            <Stat label="总发货量" value={fmtKg(st.total_shipped_kg)} cls="blue" />
            <Stat label="销售总额" value={fmtMoney(st.total_sales)} cls="amber" />
            <Stat label="总赔付" value={fmtMoney(st.total_compensation)} cls="red" />
            <Stat label="地块总成本" value={fmtMoney(st.total_cost)} />
            <Stat label="平均满意度" value={st.avg_satisfaction ? st.avg_satisfaction + ' / 5' : '—'} />
            <Stat label="续认率" value={st.renew_rate_pct != null ? st.renew_rate_pct + '%' : '—'} cls="amber" />
            <Stat label="批次" value={st.batch_no} />
          </div>
          <p className="mt16">{st.summary}</p>
        </div>
      ) : (
        <Empty text="本季尚未结算" />
      )}
      <h3 className="mt16">🛡 保险理赔</h3>
      <table>
        <thead><tr><th>保单号</th><th>类型</th><th>损失</th><th>理赔额</th><th>状态</th><th>备注</th></tr></thead>
        <tbody>
          {claims.map((c) => (
            <tr key={c.id}><td>{c.policy_no}</td><td>{c.kind}</td><td>{fmtKg(c.loss_kg)}</td>
              <td>{fmtMoney(c.amount)}</td><td><Tag cls={c.status === 'settled' ? 'green' : 'amber'}>{c.status}</Tag></td>
              <td className="small muted">{c.note}</td></tr>
          ))}
        </tbody>
      </table>
      {claims.length === 0 && <Empty text="暂无理赔记录" />}
    </>
  )
}

// ---------- 成熟度分批发货 ----------

const PLAN_CLS = {
  draft: 'gray', submitted: 'amber', pack_confirmed: 'red',
  adopter_confirmed: 'blue', archived: 'green', cancelled: 'gray',
}

function DeliveryPlansTab({ d, canOperate, onReload }) {
  const plans = d.delivery_plans || []
  if (plans.length === 0) {
    return <Empty text="本批次暂无成熟度分批方案。同一批次成熟不均时，可由农场按成熟度拆成多轮，逐轮写明预计重量与口感差异。" />
  }
  return (
    <div className="grid">
      {plans.map((pl) => <PlanManageCard key={pl.id} plan={pl} canOperate={canOperate} onReload={onReload} />)}
    </div>
  )
}

function PlanManageCard({ plan, canOperate, onReload }) {
  const toast = useToast()
  const { user } = useAuth()
  const role = user?.role
  const isFarmer = role === 'farmer' || role === 'admin'
  const isPacker = role === 'packer' || isFarmer
  const act = async (path, body) => {
    try { await post(path, body || {}); toast('操作成功'); onReload() } catch (e) { toast(e.message, true) }
  }
  const total = plan.rounds.reduce((s, r) => s + Number(r.est_weight_kg || 0), 0)
  return (
    <div className="plan-box">
      <div className="row">
        <h3 style={{ margin: 0 }}>订单 #{plan.adoption_id} · {plan.adopter_name || plan.recipient_name}（{plan.shares} 份）</h3>
        <Tag cls={PLAN_CLS[plan.status]}>{PLAN_STATUS_LABEL[plan.status] || plan.status}</Tag>
        <span className="spacer" />
        <span className="small muted">共 {plan.rounds.length} 轮 · 预计 {total} kg</span>
      </div>
      {plan.reason && <p className="small muted mt8">{plan.reason}</p>}
      <table className="mt8">
        <thead>
          <tr><th>轮次</th><th>计划采摘</th><th>预计重量</th><th>口感差异</th><th>成熟度</th><th>采摘工/状态</th><th>运单状态</th></tr>
        </thead>
        <tbody>
          {plan.rounds.map((r) => (
            <tr key={r.id}>
              <td><strong>第{r.seq}轮</strong></td>
              <td>{fmtDay(r.planned_date)}</td>
              <td>{r.est_weight_kg} kg</td>
              <td className="small">{r.taste_diff}</td>
              <td className="small muted">{r.maturity_note || '—'}</td>
              <td className="small">
                {r.picker_name ? `${r.picker_name}` : <span className="muted">未安排</span>}
                {r.harvest_status === 'done' && <Tag cls="gray">已采{r.actual_kg != null ? ` ${r.actual_kg}kg` : ''}</Tag>}
              </td>
              <td>{r.shipment_status ? <ShipFlow status={r.shipment_status} /> : <span className="small muted">待确认</span>}</td>
            </tr>
          ))}
        </tbody>
      </table>

      {canOperate && (
        <div className="row mt8">
          {plan.status === 'draft' && isFarmer && (
            <button className="sm" onClick={() => act(`/delivery-plans/${plan.id}/submit`)}>📤 提交采摘计划（同步包装组）</button>
          )}
          {plan.status === 'submitted' && isPacker && (
            <button className="sm" onClick={() => act(`/delivery-plans/${plan.id}/pack-confirm`)}>✅ 包装组确认采摘计划</button>
          )}
          {plan.status === 'adopter_confirmed' && isFarmer && (
            <button className="sm ghost" onClick={() => act(`/delivery-plans/${plan.id}/archive`)}>📦 权益归档（全部送达后）</button>
          )}
        </div>
      )}
      {canOperate && isFarmer && plan.status === 'adopter_confirmed' && (
        <RoundPickerAssign plan={plan} onReload={onReload} />
      )}
    </div>
  )
}

function RoundPickerAssign({ plan, onReload }) {
  const toast = useToast()
  const [rid, setRid] = useState(plan.rounds[0]?.id || '')
  const [picker, setPicker] = useState(4)
  const [date, setDate] = useState('')
  const submit = async () => {
    try {
      await post(`/delivery-plans/${plan.id}/rounds/${rid}/picker`,
        { picker_id: Number(picker), planned_date: date || undefined })
      toast('已安排该轮采摘工人')
      onReload()
    } catch (e) { toast(e.message, true) }
  }
  return (
    <div className="row mt8 small">
      <strong>安排下一轮采摘工：</strong>
      <select value={rid} onChange={(e) => setRid(e.target.value)}>
        {plan.rounds.map((r) => <option key={r.id} value={r.id}>第{r.seq}轮（{r.est_weight_kg}kg）</option>)}
      </select>
      <input type="number" style={{ width: 110 }} placeholder="采摘工ID" value={picker} onChange={(e) => setPicker(e.target.value)} />
      <input type="date" value={date} onChange={(e) => setDate(e.target.value)} />
      <button className="sm" onClick={submit}>安排</button>
      <span className="muted">默认采摘工 赵秀兰(ID 4)；采摘计划随分轮同步到包装组。</span>
    </div>
  )
}

// ---------- 操作弹窗 ----------
function PlanCreateForm({ projectId, adoptions, onDone, onClose }) {
  const toast = useToast()
  const [adoptionId, setAdoptionId] = useState(adoptions[0]?.id || '')
  const [reason, setReason] = useState('')
  const [rounds, setRounds] = useState([
    { planned_date: '', est_weight_kg: '', taste_diff: '', maturity_note: '', pack_spec: '', cold_route: '' },
    { planned_date: '', est_weight_kg: '', taste_diff: '', maturity_note: '', pack_spec: '', cold_route: '' },
  ])
  const setR = (i, k, v) => setRounds(rounds.map((r, idx) => idx === i ? { ...r, [k]: v } : r))
  const addRound = () => setRounds([...rounds, { planned_date: '', est_weight_kg: '', taste_diff: '', maturity_note: '', pack_spec: '', cold_route: '' }])
  const total = rounds.reduce((s, r) => s + Number(r.est_weight_kg || 0), 0)

  const submit = async () => {
    if (!adoptionId) { toast('请选择认养订单', true); return }
    if (rounds.length < 2) { toast('至少 2 轮发货', true); return }
    for (let i = 0; i < rounds.length; i++) {
      if (!(Number(rounds[i].est_weight_kg) > 0)) { toast(`第${i + 1}轮预计重量必填`, true); return }
      if (!rounds[i].taste_diff.trim()) { toast(`第${i + 1}轮需说明口感差异`, true); return }
    }
    try {
      await post(`/projects/${projectId}/delivery-plans`, {
        adoption_id: Number(adoptionId),
        reason,
        rounds: rounds.map((r, i) => ({
          seq: i + 1,
          est_weight_kg: Number(r.est_weight_kg),
          taste_diff: r.taste_diff,
          maturity_note: r.maturity_note,
          planned_date: r.planned_date,
          pack_spec: r.pack_spec,
          cold_route: r.cold_route,
        })),
      })
      toast('方案已编制（编制中），提交后将同步包装组确认')
      onDone(); onClose()
    } catch (e) { toast(e.message, true) }
  }

  return (
    <>
      <p className="muted small">同一认养批次成熟不均时，按成熟度拆成多轮采摘发货，逐轮写明预计重量与口感差异；认养人确认后自动同步物流计划、采摘任务与权益核算。</p>
      <label>认养订单</label>
      <select value={adoptionId} onChange={(e) => setAdoptionId(e.target.value)}>
        {adoptions.length === 0 && <option value="">（暂无可选订单）</option>}
        {adoptions.map((a) => (
          <option key={a.id} value={a.id}>订单#{a.id} · {a.recipient_name}（{a.shares}份 · {a.status}）</option>
        ))}
      </select>
      <label>成熟不均情况说明</label>
      <textarea rows="2" value={reason} onChange={(e) => setReason(e.target.value)}
        placeholder="如：阳面果已上糖全红，内膛果尚青，为保证各轮赏味期拆分发货" />
      <div className="row mt8"><strong>发货轮次</strong><span className="spacer" />
        <span className="small muted">合计预计 {total} kg</span>
        <button className="sm ghost" onClick={addRound}>＋ 增加一轮</button>
      </div>
      {rounds.map((r, i) => (
        <div key={i} className="plan-round-edit mt8">
          <div className="row"><strong>第 {i + 1} 轮</strong><span className="spacer" />
            {rounds.length > 2 && <button className="sm ghost" onClick={() => setRounds(rounds.filter((_, idx) => idx !== i))}>删除</button>}
          </div>
          <div className="grid grid-3">
            <div><label>计划采摘日期</label><input type="date" value={r.planned_date} onChange={(e) => setR(i, 'planned_date', e.target.value)} /></div>
            <div><label>预计重量 kg</label><input type="number" value={r.est_weight_kg} onChange={(e) => setR(i, 'est_weight_kg', e.target.value)} /></div>
            <div><label>包装规格</label><input value={r.pack_spec} onChange={(e) => setR(i, 'pack_spec', e.target.value)} placeholder="精品大果80-85mm/12枚" /></div>
          </div>
          <label>口感差异（糖度/脆面/香气）</label>
          <input value={r.taste_diff} onChange={(e) => setR(i, 'taste_diff', e.target.value)}
            placeholder="如：糖度约14.5%，脆甜多汁、着色≥90%、果香最浓" />
          <label>成熟度说明</label>
          <input value={r.maturity_note} onChange={(e) => setR(i, 'maturity_note', e.target.value)}
            placeholder="如：淀粉染色1-2级已全熟，先采阳面树冠" />
          <label>冷链路线</label>
          <input value={r.cold_route} onChange={(e) => setR(i, 'cold_route', e.target.value)}
            placeholder="烟台-济南分拨-杭州（全程0-4℃）" />
        </div>
      ))}
      <div className="mt16"><button onClick={submit}>编制方案</button></div>
    </>
  )
}

function ActionModal({ kind, projectId, onClose, onDone }) {
  const toast = useToast()
  const simple = typeof kind === 'string'
  const type = simple ? kind : kind.type
  const titles = {
    log: '记录农事 / 灾害', check: '成熟度抽检', harvest: '安排采摘任务', sorting: '登记分拣',
    shipment: '编排序批发货', plan: '按成熟度编制多轮发货', cost: '地块记账', live: '创建直播', settle: '季末结算',
  }
  const [f, setF] = useState({})
  const set = (k, v) => setF({ ...f, [k]: v })
  const submit = async () => {
    try {
      await dispatchAction(type, projectId, f, kind.task || kind.ship)
      toast('操作成功')
      onDone()
      onClose()
    } catch (e) {
      toast(e.message, true)
    }
  }
  return (
    <Modal title={titles[type] || '操作'} onClose={onClose} wide={type === 'plan'}>
      {type === 'plan' && (
        <PlanCreateForm projectId={projectId} adoptions={kind.adoptions || []} onDone={onDone} onClose={onClose} />
      )}
      {type === 'log' && (
        <>
          <label>类型</label>
          <select onChange={(e) => set('log_type', e.target.value)} defaultValue="farming">
            <option value="farming">农事</option><option value="fertilizer">施肥</option>
            <option value="pesticide">用药</option><option value="disaster">⚠️ 天气灾害</option>
            <option value="photo">照片</option><option value="other">其他</option>
          </select>
          <label>标题</label><input onChange={(e) => set('title', e.target.value)} placeholder="如：遭遇冰雹灾害" />
          <label>内容</label><textarea rows="3" onChange={(e) => set('content', e.target.value)} />
          {f.log_type === 'disaster' && (
            <div className="grid grid-2">
              <div><label>灾害类型</label><input onChange={(e) => set('disaster_kind', e.target.value)} placeholder="冰雹/连阴雨/大风…" /></div>
              <div><label>预估产量影响 %</label><input type="number" onChange={(e) => set('yield_impact_pct', Number(e.target.value))} /></div>
            </div>
          )}
        </>
      )}
      {type === 'check' && (
        <div className="grid grid-2">
          <div><label>糖度</label><input type="number" step="0.1" onChange={(e) => set('brix', Number(e.target.value))} /></div>
          <div><label>样本数</label><input type="number" onChange={(e) => set('sample_count', Number(e.target.value))} /></div>
          <div><label>成熟度 %</label><input type="number" onChange={(e) => set('maturity_pct', Number(e.target.value))} /></div>
          <div><label>预估产量 kg</label><input type="number" onChange={(e) => set('est_yield_kg', Number(e.target.value))} /></div>
          <div><label>坏果率 %</label><input type="number" step="0.1" onChange={(e) => set('bad_rate_pct', Number(e.target.value))} /></div>
          <div><label>分级（JSON）</label><input defaultValue='{"large":55,"mid":35,"small":10}' onChange={(e) => { try { set('grade_dist', JSON.parse(e.target.value)) } catch {} }} /></div>
          <div style={{ gridColumn: '1/-1' }}><label>备注</label><input onChange={(e) => set('notes', e.target.value)} /></div>
        </div>
      )}
      {type === 'harvest' && (
        <div className="grid grid-2">
          <div><label>采摘工 ID</label><input type="number" onChange={(e) => set('picker_id', Number(e.target.value))} /></div>
          <div><label>优先级（越小越优先）</label><input type="number" defaultValue={100} onChange={(e) => set('priority', Number(e.target.value))} /></div>
          <div><label>计划日期</label><input type="date" onChange={(e) => set('planned_date', e.target.value)} /></div>
          <div><label>目标产量 kg</label><input type="number" onChange={(e) => set('target_kg', Number(e.target.value))} /></div>
        </div>
      )}
      {type === 'harvest_done' && (
        <div className="grid grid-2">
          <div><label>实际采摘 kg</label><input type="number" onChange={(e) => set('actual_kg', Number(e.target.value))} /></div>
          <div><label>坏果率 %</label><input type="number" step="0.1" onChange={(e) => set('bad_rate_pct', Number(e.target.value))} /></div>
        </div>
      )}
      {type === 'sorting' && (
        <div className="grid grid-3">
          <div style={{ gridColumn: '1/-1' }}><label>包装规格</label><input onChange={(e) => set('spec', e.target.value)} placeholder="如：精品大果80-85mm/12枚装" /></div>
          <div><label>重量 kg</label><input type="number" onChange={(e) => set('weight_kg', Number(e.target.value))} /></div>
          <div><label>箱数</label><input type="number" onChange={(e) => set('box_count', Number(e.target.value))} /></div>
        </div>
      )}
      {type === 'shipment' && (
        <>
          <div className="grid grid-2">
            <div><label>认养订单 ID</label><input type="number" onChange={(e) => set('adoption_id', Number(e.target.value))} /></div>
            <div><label>第几批</label><input type="number" defaultValue={1} onChange={(e) => set('seq', Number(e.target.value))} /></div>
            <div><label>计划日期</label><input type="date" onChange={(e) => set('plan_date', e.target.value)} /></div>
            <div><label>重量 kg</label><input type="number" onChange={(e) => set('weight_kg', Number(e.target.value))} /></div>
          </div>
          <label>包装规格</label><input onChange={(e) => set('pack_spec', e.target.value)} />
          <label>冷链路线</label><input onChange={(e) => set('cold_route', e.target.value)} placeholder="烟台-济南分拨-北京（全程0-4℃）" />
        </>
      )}
      {type === 'ship_advance' && (
        <ShipAdvanceForm ship={kind.ship} />
      )}
      {type === 'cost' && (
        <div className="grid grid-2">
          <div><label>成本类型</label>
            <select onChange={(e) => set('cost_type', e.target.value)} defaultValue="">
              <option value="" disabled>请选择</option>
              <option value="input">农事投入</option><option value="disaster">防灾/灾害</option>
              <option value="harvest">采摘人工</option><option value="sorting">分拣包装</option>
              <option value="logistics">冷链物流</option><option value="compensation">赔付</option>
            </select>
          </div>
          <div><label>金额 ¥</label><input type="number" onChange={(e) => set('amount', Number(e.target.value))} /></div>
          <div style={{ gridColumn: '1/-1' }}><label>事项</label><input onChange={(e) => set('item', e.target.value)} /></div>
        </div>
      )}
      {type === 'live' && (
        <>
          <label>直播标题</label><input onChange={(e) => set('title', e.target.value)} />
          <label>推流/回放地址</label><input onChange={(e) => set('stream_url', e.target.value)} placeholder="https://live.example.com/…" />
          <label>开始时间</label><input type="datetime-local" onChange={(e) => set('starts_at', e.target.value)} />
        </>
      )}
      {type === 'settle' && (
        <>
          <p className="muted">系统将自动汇总实际采摘、发货、销售额、成本、赔付、满意度与续认率。可补充本季总结与下一季建议。</p>
          <label>季度总结 / 下一季续认建议</label>
          <textarea rows="4" onChange={(e) => set('summary', e.target.value)} placeholder="如：本季受冰雹影响…建议增设防雹网、上线净重磅识模板…" />
        </>
      )}
      {type !== 'ship_advance' && type !== 'plan' && <div className="mt16"><button onClick={submit}>提交</button></div>}
    </Modal>
  )
}

function ShipAdvanceForm({ ship }) {
  const toast = useToast()
  const next = { planned: 'picked', picked: 'packed', packed: 'shipped', shipped: 'delivered' }[ship.status]
  const label = { picked: '确认已采摘', packed: '确认已包装', shipped: '物流发车', delivered: '确认送达' }[next]
  const [f, setF] = useState({ status: next })
  return (
    <>
      <p>当前：<ShipFlow status={ship.status} />，下一步：<strong>{label}</strong></p>
      {next === 'shipped' && (
        <div className="grid grid-2">
          <div><label>物流公司</label><input onChange={(e) => setF({ ...f, logistics_company: e.target.value })} /></div>
          <div><label>运单号</label><input onChange={(e) => setF({ ...f, tracking_no: e.target.value })} /></div>
          <div style={{ gridColumn: '1/-1' }}><label>发货坏果率 %</label><input type="number" step="0.1" onChange={(e) => setF({ ...f, bad_fruit_rate: Number(e.target.value) })} /></div>
        </div>
      )}
      <button className="mt16" onClick={async () => {
        try { await post(`/projects/${ship.project_id}/shipments/${ship.id}/advance`, f); toast('状态已推进'); location.reload() }
        catch (e) { toast(e.message, true) }
      }}>{label}</button>
    </>
  )
}

async function dispatchAction(type, projectId, f, extra) {
  if (type === 'harvest_done') return post(`/projects/${projectId}/harvests/${extra.id}/complete`, f)
  const paths = {
    log: 'logs', check: 'checks', harvest: 'harvests', sorting: 'sorting',
    shipment: 'shipments', cost: 'costs', live: 'live', settle: 'settle',
  }
  return post(`/projects/${projectId}/${paths[type]}`, f)
}
