const state = { cases: [], workbench: null, selected: null };
const $ = (id) => document.getElementById(id);
const statusNames = { draft: '草拟', baseline_frozen: '基线已冻结', remediation_required: '待整改', candidate_ready: '候选版就绪', under_review: '待复核', returned: '退回整改', sealed: '已封存' };

async function api(path, options = {}) {
  const response = await fetch(path, options);
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(body.error?.message || `请求失败 (${response.status})`);
  return body;
}

function requestID(prefix) {
  return `${prefix}-${Date.now()}-${crypto.getRandomValues(new Uint32Array(1))[0].toString(16)}`;
}

function meta(actor) {
  return { actor_id: actor, request_id: requestID('ui'), expected_revision: state.workbench.case.revision };
}

function post(path, payload) {
  return api(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) });
}

async function loadCases() {
  const data = await api('/api/cases');
  state.cases = data.cases;
  renderCaseList();
}

async function selectCase(caseID) {
  state.selected = caseID;
  state.workbench = await api(`/api/cases/${encodeURIComponent(caseID)}`);
  $('emptyState').classList.add('hidden');
  $('workbench').classList.remove('hidden');
  renderCaseList();
  renderWorkbench();
}

function renderCaseList() {
  $('caseList').replaceChildren(...state.cases.map(item => {
    const button = document.createElement('button');
    button.className = `case-item${item.case_id === state.selected ? ' active' : ''}`;
    button.type = 'button';
    button.innerHTML = `<strong>${escapeHTML(item.title)}</strong><span>${escapeHTML(statusNames[item.status] || item.status)} · r${item.revision}</span>`;
    button.addEventListener('click', () => selectCase(item.case_id).catch(showError));
    return button;
  }));
}

function renderWorkbench() {
  const w = state.workbench, c = w.case;
  $('caseStatus').textContent = statusNames[c.status] || c.status;
  $('caseRevision').textContent = `修订 r${c.revision}`;
  $('caseTitle').textContent = c.title;
  $('caseMeta').textContent = `${c.collection_date} · ${c.custody_reference} · 档案员 ${c.archivist_id} / 复核员 ${c.reviewer_id}`;
  $('baselineInfo').textContent = c.consent_baseline_sha256 ? `已冻结 · ${c.consent_baseline_sha256}` : `录音 ${c.source_sha256} · 授权文书 ${c.consent_document_sha256}`;
  renderSegments(c.segments || []);
  renderConstraints(c.constraints || []);
  renderCoverage(w.coverage);
  renderConflicts(c.conflicts || []);
  renderCheck(w.latest_check, w.check_stale);
  renderCandidate(c.candidate);
  renderReview(c, w.verification);
  renderReturnTasks(c.return_tasks || [], w.review_difference);
  renderManifest(w.manifest_query);
  renderTimeline(w.timeline || []);
  applyActions(new Set(w.allowed_actions || []));
}

function renderCoverage(matrix) {
  const summary = matrix?.summary || {};
  $('coverageSummary').innerHTML = [['明确', summary.clear || 0], ['未覆盖', summary.uncovered || 0], ['歧义', summary.ambiguous || 0], ['期限已届满', summary.embargo_elapsed || 0]].map(([name, count]) => `<span><strong>${count}</strong>${name}</span>`).join('');
  $('coverageRows').innerHTML = matrix?.items?.length ? matrix.items.map(item => `<tr data-segment-row="${escapeHTML(item.segment_id)}"><td>${escapeHTML(item.segment_id)}<br><span class="tag">${formatMS(item.start_ms)}</span></td><td>${escapeHTML(item.scope_type)} · ${escapeHTML(item.scope_value)}</td><td><span class="diagnostic ${escapeHTML(item.status)}">${escapeHTML(item.diagnostic_code)}</span></td><td>${item.constraints.map(c => `<span class="constraint-link">${escapeHTML(c.constraint_id)} · ${escapeHTML(c.policy)} · ${escapeHTML(c.evidence_reference)}</span>`).join('') || '—'}</td></tr>`).join('') : '<tr><td colspan="4" class="muted">登记片段后显示覆盖范围</td></tr>';
}

function renderCheck(check, stale) {
  $('checkSummary').className = stale ? 'warning-text' : '';
  $('checkSummary').textContent = check ? `${stale ? '检查已过期，请重新运行。' : '当前检查有效。'} 输入 ${check.input_sha256} · 结果 ${check.result_sha256} · 基于 r${check.based_on_revision}` : '尚未运行冲突检查。';
  const delta = check?.delta || {};
  $('checkDelta').innerHTML = [['新增', delta.new?.length || 0], ['重开', delta.reopened?.length || 0], ['已解决', delta.resolved?.length || 0], ['未变化', delta.unchanged?.length || 0], ['阻断', check?.blocking_count || 0], ['提示', check?.notice_count || 0]].map(([name, count]) => `<span><strong>${count}</strong>${name}</span>`).join('');
}

function renderSegments(items) {
  $('segmentRows').innerHTML = items.length ? items.map(s => `<tr><td>${formatMS(s.start_ms)}–${formatMS(s.end_ms)}</td><td>${escapeHTML(s.source_text)}</td><td>${escapeHTML([...(s.subject_ids || []), ...(s.topic_codes || [])].join(', '))}</td><td>${escapeHTML(s.disposition)}</td></tr>`).join('') : '<tr><td colspan="4" class="muted">尚未登记片段</td></tr>';
}

function renderConstraints(items) {
  $('constraintRows').innerHTML = items.length ? items.map(c => `<tr><td>${escapeHTML(c.scope_type)} · ${escapeHTML(c.scope_value)}</td><td>${escapeHTML(c.policy)}${c.required_alias ? ` → ${escapeHTML(c.required_alias)}` : ''}</td><td>${escapeHTML(c.evidence_reference)}</td></tr>`).join('') : '<tr><td colspan="3" class="muted">尚未登记约束</td></tr>';
}

function renderConflicts(items) {
  $('conflictList').innerHTML = items.length ? items.map(item => `<article class="conflict ${item.severity}"><div><span class="tag">${escapeHTML(item.conflict_id)}</span><br><strong>${escapeHTML(item.segment_id)}</strong></div><div><strong>${escapeHTML(item.code)}</strong><p>${escapeHTML(item.message)}</p></div><span>${item.resolved ? '已处置' : item.severity === 'blocking' ? '阻断' : '提示'}</span>${!item.resolved && item.severity === 'blocking' ? `<button data-remediate="${escapeHTML(item.segment_id)}" type="button">提交整改</button>` : ''}</article>`).join('') : '<p class="muted">运行冲突检查后显示结果。</p>';
  document.querySelectorAll('[data-remediate]').forEach(button => button.addEventListener('click', () => openRemediation(button.dataset.remediate)));
}

function renderCandidate(candidate) {
  $('candidateDigest').textContent = candidate ? `修订 ${candidate.revision} · ${candidate.content_sha256}` : '尚未生成';
  $('diffList').innerHTML = candidate ? candidate.diffs.map(d => `<article class="diff"><div><strong>${escapeHTML(d.segment_id)}</strong><br><span class="tag">${escapeHTML(d.action)}</span></div><div class="diff-text"><div>${escapeHTML(d.source_text)}</div><div>${escapeHTML(d.public_text || '〔整段排除〕')}</div></div><span>${d.changed ? '有变更' : '原样公开'}</span></article>`).join('') : '<p class="muted">处置全部阻断项后生成候选版。</p>';
  $('audioList').innerHTML = candidate?.audio_instructions?.length ? candidate.audio_instructions.map(i => `<div class="instruction"><strong>${escapeHTML(i.action)}</strong> · ${formatMS(i.start_ms)}–${formatMS(i.end_ms)} · ${escapeHTML(i.reason)}</div>`).join('') : '<span class="muted">无音频处理指令</span>';
}

function renderReview(c, verification) {
  const items = c.candidate?.diffs || [];
  $('reviewItems').innerHTML = items.map(item => `<div class="review-item" data-review-segment="${escapeHTML(item.segment_id)}"><strong>${escapeHTML(item.segment_id)}</strong><label><input type="checkbox" data-consent> 授权有效</label><label><input type="checkbox" data-redaction> 遮蔽有效</label></div>`).join('') || '<p class="muted">候选版送审后可逐项确认。</p>';
  if (verification) {
    $('verification').className = `verification${verification.valid ? '' : ' invalid'}`;
    const checks = (verification.digest_checks || []).map(item => `<div>${item.valid ? '通过' : '失败'} · ${escapeHTML(item.name)} <span class="tag">${escapeHTML(item.code)}</span><br><small>实际 ${escapeHTML(item.actual)} · 封存 ${escapeHTML(item.sealed)}</small></div>`).join('');
    const issues = (verification.issues || []).map(item => `<div class="warning-text">${escapeHTML(item.code)}${item.segment_id ? ` · ${escapeHTML(item.segment_id)}` : ''} · ${escapeHTML(item.message)}</div>`).join('');
    $('verification').innerHTML = `<strong>${verification.valid ? '校验通过' : '校验失败，停止使用'}</strong><p>${escapeHTML(verification.message)}</p>${checks}${issues}`;
  } else {
    $('verification').className = 'verification muted';
    $('verification').textContent = '批准后显示校验结果';
  }
}

function renderReturnTasks(tasks, difference) {
  $('returnTasks').innerHTML = tasks.length ? tasks.map(task => `<div><strong>${escapeHTML(task.task_id)}</strong> · 第 ${task.round} 轮 · ${escapeHTML(task.segment_id)} / ${escapeHTML(task.code)}<br><span>${escapeHTML(task.status)} · ${escapeHTML(task.comment)}</span></div>`).join('') : '<span class="muted">暂无退回任务</span>';
  $('roundDiff').innerHTML = difference?.changes?.length ? difference.changes.map(change => `<div class="${change.anomaly ? 'warning-text' : ''}"><strong>${escapeHTML(change.segment_id)}</strong> · ${escapeHTML(change.kind)}${change.anomaly ? ' · 异常变化' : ''}</div>`).join('') : '<span class="muted">暂无可比较轮次</span>';
}

function renderManifest(result) {
  $('manifestDetails').classList.toggle('hidden', !state.workbench.verification);
  if (!result) { $('manifestEntries').innerHTML = ''; return; }
  const publicRows = (result.public_segments || []).map(item => `<div><strong>公开 · ${escapeHTML(item.segment_id)}</strong> · ${formatMS(item.start_ms)}–${formatMS(item.end_ms)}<br>${escapeHTML(item.text)}</div>`);
  const excluded = (result.excluded_ranges || []).map(item => `<div><strong>排除</strong> · ${formatMS(item.start_ms)}–${formatMS(item.end_ms)}</div>`);
  const audio = (result.audio_instructions || []).map(item => `<div><strong>音频 · ${escapeHTML(item.segment_id)}</strong> · ${escapeHTML(item.action)} · ${formatMS(item.start_ms)}–${formatMS(item.end_ms)}</div>`);
  $('manifestEntries').innerHTML = [...publicRows, ...excluded, ...audio].join('') || '<span class="muted">没有符合条件的清单条目</span>';
}

function renderTimeline(items) {
  $('timeline').innerHTML = items.map(item => `<li><strong>${escapeHTML(item.type)}</strong> · ${escapeHTML(item.actor_id)}<small>r${item.revision} · ${new Date(item.occurred_at).toLocaleString('zh-CN')} · ${escapeHTML(item.event_sha256)}</small></li>`).join('');
}

function applyActions(actions) {
  const mapping = { freezeButton: 'freeze', batchButton: 'register_batch', addSegmentButton: 'add_segment', addConstraintButton: 'add_constraint', checkButton: 'check_conflicts', generateButton: 'generate_candidate', submitButton: 'submit_review', returnButton: 'return_review', approveButton: 'approve' };
  Object.entries(mapping).forEach(([id, action]) => $(id).disabled = !actions.has(action));
}

function escapeHTML(value = '') {
  return String(value).replace(/[&<>"]/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[char]);
}

function formatMS(ms) {
  const minutes = Math.floor(ms / 60000).toString().padStart(2, '0');
  const seconds = Math.floor((ms % 60000) / 1000).toString().padStart(2, '0');
  const millis = (ms % 1000).toString().padStart(3, '0');
  return `${minutes}:${seconds}.${millis}`;
}

const field = (name, label, type = 'text', wide = false, value = '') => `<label class="${wide ? 'wide' : ''}">${label}<input name="${name}" type="${type}" value="${escapeHTML(value)}" required></label>`;
const area = (name, label) => `<label class="wide">${label}<textarea name="${name}" required></textarea></label>`;

function openDialog(title, fields, submit) {
  $('dialogTitle').textContent = title;
  $('dialogFields').innerHTML = fields;
  $('dialogError').textContent = '';
  $('dialogPreview').className = 'dialog-preview hidden';
  $('dialogPreview').innerHTML = '';
  $('dialogSubmit').disabled = false;
  const dialog = $('formDialog');
  const handler = async event => {
    if (event.submitter?.value === 'cancel') return;
    event.preventDefault();
    try {
      const data = Object.fromEntries(new FormData($('dialogForm')));
      await submit(data);
      dialog.close();
      await afterMutation();
    } catch (error) { $('dialogError').textContent = error.message; }
  };
  $('dialogForm').onsubmit = handler;
  dialog.showModal();
}

function parseJSONLines(value, label) {
  return value.split('\n').map(line => line.trim()).filter(Boolean).map((line, index) => {
    try { return JSON.parse(line); } catch (_) { throw new Error(`${label}第 ${index + 1} 行不是有效 JSON`); }
  });
}

function openBatch() {
  const fields = '<label class="wide">片段（每行一个 JSON 对象）<textarea name="segments" class="code-input" required></textarea></label><label class="wide">授权约束（每行一个 JSON 对象）<textarea name="constraints" class="code-input" required></textarea></label><button id="batchPreviewButton" type="button" class="secondary wide">运行批量预检</button>';
  openDialog('片段与授权约束批量登记', fields, async data => {
    const segments = parseJSONLines(data.segments, '片段');
    const constraints = parseJSONLines(data.constraints, '约束');
    const result = await post(`/api/cases/${state.selected}/registration-batch`, { ...meta(state.workbench.case.archivist_id), preview: false, segments, constraints });
    if (!result.valid) throw new Error(result.issues.map(i => `${i.record_type} 第${i.line}行 ${i.field}: ${i.message}`).join('；'));
  });
  $('dialogSubmit').disabled = true;
  $('dialogForm').addEventListener('input', () => { $('dialogSubmit').disabled = true; });
  $('batchPreviewButton').onclick = async () => {
    try {
      const data = Object.fromEntries(new FormData($('dialogForm')));
      const payload = { ...meta(state.workbench.case.archivist_id), preview: true, segments: parseJSONLines(data.segments, '片段'), constraints: parseJSONLines(data.constraints, '约束') };
      const result = await post(`/api/cases/${state.selected}/registration-batch`, payload);
      $('dialogPreview').className = `dialog-preview${result.valid ? '' : ' invalid'}`;
      $('dialogPreview').innerHTML = result.valid ? `预检通过：片段新增 ${result.summary.segments_added}、更新 ${result.summary.segments_updated}；约束新增 ${result.summary.constraints_added}、更新 ${result.summary.constraints_updated}；覆盖 ${formatMS(result.summary.coverage_start_ms)}–${formatMS(result.summary.coverage_end_ms)}` : result.issues.map(i => `<div>${escapeHTML(i.record_type)} 第 ${i.line} 行 · ${escapeHTML(i.field)} · ${escapeHTML(i.message)}</div>`).join('');
      $('dialogSubmit').disabled = !result.valid;
    } catch (error) { $('dialogError').textContent = error.message; }
  };
}

function openNewCase() {
  openDialog('建立口述史案件', field('case_id', '案件标识') + field('title', '题名') + field('collection_date', '采集日期', 'date') + field('custody_reference', '保管标识') + field('source_audio_uri', '原始录音引用', 'text', true) + field('source_sha256', '录音 SHA-256', 'text', true) + field('consent_document_sha256', '授权文书 SHA-256', 'text', true) + field('archivist_id', '档案员') + field('reviewer_id', '复核员'), async data => {
    const payload = { ...data, actor_id: data.archivist_id, request_id: requestID('create'), expected_revision: 0 };
    const result = await post('/api/cases', payload);
    state.selected = result.case.case_id;
  });
}

function openSegment() {
  openDialog('登记文字片段', field('segment_id', '片段标识') + field('start_ms', '开始毫秒', 'number') + field('end_ms', '结束毫秒', 'number') + field('subject_ids', '人物标识（逗号分隔）') + field('topic_codes', '主题代码（逗号分隔）') + area('source_text', '原始文字'), async data => {
    const segment = { segment_id: data.segment_id, start_ms: Number(data.start_ms), end_ms: Number(data.end_ms), source_text: data.source_text, subject_ids: split(data.subject_ids), topic_codes: split(data.topic_codes), disposition: 'original' };
    await post(`/api/cases/${state.selected}/segments`, { ...meta(state.workbench.case.archivist_id), segment });
  });
}

function openConstraint() {
  const fields = field('constraint_id', '约束标识') + '<label>范围类型<select name="scope_type"><option value="subject">人物</option><option value="topic">主题</option></select></label>' + field('scope_value', '范围值') + '<label>公开策略<select name="policy"><option value="allow">允许公开</option><option value="anonymous">必须匿名</option><option value="deny">禁止公开</option><option value="delay">延后公开</option></select></label>' + field('required_alias', '要求别名（匿名时）') + field('not_before', '最早公开日期（延后时）', 'date') + field('evidence_reference', '授权依据', 'text', true);
  openDialog('登记授权约束', fields, async data => {
    const constraint = { constraint_id: data.constraint_id, scope_type: data.scope_type, scope_value: data.scope_value, policy: data.policy, required_alias: data.required_alias, not_before: data.not_before, evidence_reference: data.evidence_reference };
    await post(`/api/cases/${state.selected}/constraints`, { ...meta(state.workbench.case.archivist_id), constraint });
  });
}

function openRemediation(segmentID) {
  const fields = field('segment_id', '片段标识', 'text', false, segmentID) + '<label>处置方式<select name="disposition"><option value="replace">替换文本</option><option value="mute">静音区间</option><option value="exclude">整段排除</option></select></label><label class="wide">公开替换文本（替换或静音时）<textarea name="public_text"></textarea></label><label>静音区间（start-end，逗号分隔）<input name="mute_ranges" type="text"></label>' + field('evidence_refs', '证据引用（逗号分隔）', 'text', true) + area('reason', '处置理由') + '<button id="remediationPreviewButton" type="button" class="secondary wide">预演处置</button>';
  openDialog('提交遮蔽整改', fields, async data => {
    await post(`/api/cases/${state.selected}/remediate`, remediationPayload(data));
  });
  $('dialogSubmit').disabled = true;
  $('dialogForm').addEventListener('input', () => { $('dialogSubmit').disabled = true; });
  $('remediationPreviewButton').onclick = async () => {
    try {
      const data = Object.fromEntries(new FormData($('dialogForm')));
      const result = await post(`/api/cases/${state.selected}/remediate`, { ...remediationPayload(data), preview: true });
      const p = result.preview;
      $('dialogPreview').className = `dialog-preview${p.can_confirm ? '' : ' invalid'}`;
      $('dialogPreview').innerHTML = `<strong>${p.can_confirm ? '可确认提交' : '仍有未满足要求'}</strong><div class="diff-text"><div>${escapeHTML(p.text_diff.source_text)}</div><div>${escapeHTML(p.text_diff.public_text || '〔整段排除〕')}</div></div>${p.audio_instructions.map(i => `<div>${escapeHTML(i.action)} · ${formatMS(i.start_ms)}–${formatMS(i.end_ms)}</div>`).join('')}${p.requirements.map(i => `<div>${i.satisfied ? '满足' : '未满足'} · ${escapeHTML(i.conflict_id)}${i.reason ? ` · ${escapeHTML(i.reason)}` : ''}</div>`).join('')}`;
      $('dialogSubmit').disabled = !p.can_confirm;
    } catch (error) { $('dialogError').textContent = error.message; }
  };
}

function remediationPayload(data) {
  const mute_ranges = data.disposition === 'mute' ? split(data.mute_ranges).map(value => { const [start, end] = value.split('-').map(Number); return { start_ms: start, end_ms: end }; }) : [];
  return { ...meta(state.workbench.case.archivist_id), segment_id: data.segment_id, disposition: data.disposition, public_text: data.public_text, mute_ranges, evidence_refs: split(data.evidence_refs), reason: data.reason };
}

function reviewItems() {
  return [...document.querySelectorAll('[data-review-segment]')].map(row => ({ segment_id: row.dataset.reviewSegment, consent_valid: row.querySelector('[data-consent]').checked, redaction_valid: row.querySelector('[data-redaction]').checked }));
}

function openReturn() {
  const fields = '<label class="wide">退回意见（每行一个 JSON 对象）<textarea name="return_reasons" class="code-input" required></textarea></label><button id="returnPreviewButton" type="button" class="secondary wide">预览任务列表</button>';
  openDialog('结构化退回复核', fields, async data => {
    const return_reasons = parseJSONLines(data.return_reasons, '退回意见');
    await post(`/api/cases/${state.selected}/review`, { ...meta(state.workbench.case.reviewer_id), item_results: reviewItems(), decision: 'returned', return_reasons });
  });
  $('dialogSubmit').disabled = true;
  $('dialogForm').addEventListener('input', () => { $('dialogSubmit').disabled = true; });
  $('returnPreviewButton').onclick = () => {
    try {
      const data = Object.fromEntries(new FormData($('dialogForm')));
      const reasons = parseJSONLines(data.return_reasons, '退回意见');
      if (!reasons.length) throw new Error('至少提供一条退回意见');
      const seen = new Set();
      reasons.forEach((item, index) => {
        if (!item.segment_id || !item.code || !item.comment) throw new Error(`第 ${index + 1} 行必须包含 segment_id、code 和 comment`);
        const key = `${item.segment_id}\u0000${item.code}`;
        if (seen.has(key)) throw new Error(`第 ${index + 1} 行与前文重复`);
        seen.add(key);
      });
      $('dialogPreview').className = 'dialog-preview';
      $('dialogPreview').innerHTML = reasons.map((item, index) => `<div>任务 ${index + 1} · ${escapeHTML(item.segment_id)} · ${escapeHTML(item.code)} · ${escapeHTML(item.comment)}</div>`).join('');
      $('dialogSubmit').disabled = false;
    } catch (error) { $('dialogError').textContent = error.message; }
  };
}

async function simpleAction(path, actor) {
  await post(`/api/cases/${state.selected}/${path}`, meta(actor));
  await afterMutation();
}

async function afterMutation() {
  await loadCases();
  if (state.selected) await selectCase(state.selected);
  showToast('操作已提交并记录审计事件');
}

function split(value) { return value.split(',').map(item => item.trim()).filter(Boolean); }
function showError(error) { showToast(error.message || String(error), true); }
function showToast(message, error = false) { const toast = $('toast'); toast.textContent = message; toast.style.background = error ? '#9e3d38' : '#17211c'; toast.classList.add('show'); setTimeout(() => toast.classList.remove('show'), 3200); }

document.querySelectorAll('.tab').forEach(tab => tab.addEventListener('click', () => {
  document.querySelectorAll('.tab, .tab-panel').forEach(item => item.classList.remove('active'));
  tab.classList.add('active');
  $(tab.dataset.tab).classList.add('active');
}));
$('newCaseButton').addEventListener('click', openNewCase);
$('refreshButton').addEventListener('click', () => selectCase(state.selected).catch(showError));
$('addSegmentButton').addEventListener('click', openSegment);
$('addConstraintButton').addEventListener('click', openConstraint);
$('batchButton').addEventListener('click', openBatch);
$('freezeButton').addEventListener('click', () => simpleAction('freeze', state.workbench.case.archivist_id).catch(showError));
$('checkButton').addEventListener('click', () => simpleAction('check-conflicts', state.workbench.case.archivist_id).catch(showError));
$('generateButton').addEventListener('click', () => simpleAction('candidate', state.workbench.case.archivist_id).catch(showError));
$('submitButton').addEventListener('click', () => simpleAction('submit-review', state.workbench.case.archivist_id).catch(showError));
$('returnButton').addEventListener('click', openReturn);
$('approveButton').addEventListener('click', async () => {
  try {
    await post(`/api/cases/${state.selected}/review`, { ...meta(state.workbench.case.reviewer_id), item_results: reviewItems(), decision: 'approved', return_reasons: [] });
    await afterMutation();
  } catch (error) { showError(error); }
});
$('manifestQueryButton').addEventListener('click', async () => {
  try {
    const start = $('manifestStart').value, end = $('manifestEnd').value;
    const query = { segment_id: $('manifestSegment').value.trim(), entry_type: $('manifestType').value };
    if (start !== '') query.start_ms = Number(start);
    if (end !== '') query.end_ms = Number(end);
    renderManifest(await post(`/api/cases/${state.selected}/manifest-query`, query));
  } catch (error) { showError(error); }
});

api('/healthz').then(() => { $('serviceState').textContent = '本地服务正常'; return loadCases(); }).catch(error => { $('serviceState').textContent = '服务不可用'; showError(error); });
