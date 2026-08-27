const app = document.getElementById('app');
const navBtns = document.querySelectorAll('.nav-btn');

navBtns.forEach(btn => btn.addEventListener('click', () => {
  navBtns.forEach(b => b.classList.remove('active'));
  btn.classList.add('active');
  if (btn.dataset.view === 'arenas') renderArenaList();
  if (btn.dataset.view === 'create') renderCreateForm();
}));

function api(path, opts) {
  return fetch('/api' + path, opts).then(async r => {
    const body = await r.json().catch(() => null);
    if (!r.ok) throw new Error((body && body.error) || r.statusText);
    return body;
  });
}

// ---------- Arena list ----------

async function renderArenaList() {
  const tpl = document.getElementById('tpl-arena-list');
  app.replaceChildren(tpl.content.cloneNode(true));
  const list = document.getElementById('arena-list');
  list.textContent = 'Loading…';
  try {
    const arenas = await api('/arenas');
    list.textContent = '';
    if (!arenas || arenas.length === 0) {
      list.innerHTML = '<p class="muted">No arenas yet. Create one to get started.</p>';
      return;
    }
    arenas.forEach(a => {
      const div = document.createElement('div');
      div.className = 'card arena-card';
      div.innerHTML = `<h3>${escapeHtml(a.name)}</h3><div class="muted">${a.matchups.length} prompts · ${a.mode}</div>`;
      div.addEventListener('click', () => renderArenaDetail(a.id));
      list.appendChild(div);
    });
  } catch (e) {
    list.textContent = 'Failed to load arenas: ' + e.message;
  }
}

// ---------- Create form ----------

function renderCreateForm() {
  const tpl = document.getElementById('tpl-create-arena');
  app.replaceChildren(tpl.content.cloneNode(true));
  const container = document.getElementById('matchup-fields');
  const addMatchup = () => container.appendChild(document.getElementById('tpl-matchup-field').content.cloneNode(true));
  addMatchup();
  document.getElementById('add-matchup').addEventListener('click', addMatchup);
  container.addEventListener('click', e => {
    if (e.target.classList.contains('remove-matchup')) {
      e.target.closest('.matchup-field').remove();
    }
  });

  document.getElementById('create-form').addEventListener('submit', async e => {
    e.preventDefault();
    const form = e.target;
    const name = form.name.value.trim();
    const mode = form.mode.value;
    const matchups = [...container.querySelectorAll('.matchup-field')].map(f => {
      const category = f.category.value.trim();
      return {
        prompt: f.prompt.value,
        category: category || null,
        candidate_a: { source_label: f.labelA.value || 'A', text: f.outputA.value },
        candidate_b: { source_label: f.labelB.value || 'B', text: f.outputB.value },
      };
    });
    try {
      const arena = await api('/arenas', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, mode, matchups }),
      });
      renderArenaDetail(arena.id);
      document.querySelectorAll('.nav-btn').forEach(b => b.classList.remove('active'));
      document.querySelector('.nav-btn[data-view="arenas"]').classList.add('active');
    } catch (err) {
      alert('Failed to create arena: ' + err.message);
    }
  });
}

// ---------- Arena detail (review + results) ----------

async function renderArenaDetail(arenaId) {
  const arena = await api('/arenas/' + arenaId);
  const tpl = document.getElementById('tpl-arena-detail');
  app.replaceChildren(tpl.content.cloneNode(true));
  app.querySelector('h2').textContent = arena.name;

  const tabBtns = app.querySelectorAll('.tab-btn');
  tabBtns.forEach(btn => btn.addEventListener('click', () => {
    tabBtns.forEach(b => b.classList.remove('active'));
    btn.classList.add('active');
    document.getElementById('tab-review').classList.toggle('hidden', btn.dataset.tab !== 'review');
    document.getElementById('tab-results').classList.toggle('hidden', btn.dataset.tab !== 'results');
    if (btn.dataset.tab === 'results') renderResults(arena);
  }));

  renderReview(arena);
}

function renderReview(arena) {
  const panel = document.getElementById('tab-review');
  panel.replaceChildren(document.getElementById('tpl-review').content.cloneNode(true));

  const reviewerInput = document.getElementById('reviewer-id');
  const savedId = localStorage.getItem('evalarena-reviewer-id');
  if (savedId) reviewerInput.value = savedId;

  let queue = arena.matchups.map(m => m.id);
  let idx = 0;
  const reviewCard = document.getElementById('review-card');

  const start = () => {
    const rid = reviewerInput.value.trim();
    if (!rid) return;
    localStorage.setItem('evalarena-reviewer-id', rid);
    reviewCard.classList.remove('hidden');
    idx = 0;
    loadCurrent();
  };
  reviewerInput.addEventListener('change', start);
  if (savedId) start();

  const pairwiseBox = document.getElementById('pairwise-candidates');
  const tournamentBox = document.getElementById('tournament-candidates');
  const isTournament = arena.mode === 'tournament';
  pairwiseBox.style.display = isTournament ? 'none' : '';
  tournamentBox.style.display = isTournament ? '' : 'none';

  async function loadCurrent() {
    if (idx >= queue.length) {
      document.getElementById('review-prompt').textContent = 'All prompts reviewed. Check the Results tab!';
      document.getElementById('text-left').textContent = '';
      document.getElementById('text-right').textContent = '';
      tournamentBox.replaceChildren();
      document.querySelector('.progress').textContent = `${queue.length}/${queue.length} done`;
      return;
    }
    const rid = reviewerInput.value.trim();
    const bm = await api(`/arenas/${arena.id}/matchups/${queue[idx]}/review?reviewer=${encodeURIComponent(rid)}`);
    document.getElementById('review-prompt').textContent = bm.Prompt;
    if (isTournament) {
      tournamentBox.replaceChildren();
      (bm.Options || []).forEach((opt, i) => {
        const div = document.createElement('div');
        div.className = 'candidate';
        div.innerHTML = `<h3>Option ${i + 1}</h3><pre></pre><button class="pick-btn" data-pick="${opt.PositionKey}">This wins</button>`;
        div.querySelector('pre').textContent = opt.Text;
        div.querySelector('.pick-btn').addEventListener('click', () => pick(opt.PositionKey));
        tournamentBox.appendChild(div);
      });
    } else {
      document.getElementById('text-left').textContent = bm.LeftText;
      document.getElementById('text-right').textContent = bm.RightText;
    }
    document.querySelector('.progress').textContent = `${idx + 1}/${queue.length}` + (bm.AlreadyVoted ? ' (already voted — voting again will replace your vote)' : '');
  }

  async function pick(winner) {
    const rid = reviewerInput.value.trim();
    if (!rid) { alert('Enter your reviewer name first'); return; }
    await api(`/arenas/${arena.id}/vote`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ matchup_id: queue[idx], reviewer_id: rid, pick: winner }),
    });
    idx++;
    loadCurrent();
  }

  panel.querySelectorAll('#pairwise-candidates .pick-btn, .review-actions .pick-btn').forEach(btn => {
    btn.addEventListener('click', () => pick(btn.dataset.pick));
  });

  panel._keyHandler = e => {
    if (reviewCard.classList.contains('hidden') || isTournament) return;
    if (e.key === 'ArrowLeft') pick('A');
    if (e.key === 'ArrowRight') pick('B');
    if (e.key === ' ') { e.preventDefault(); pick('tie'); }
  };
  document.addEventListener('keydown', panel._keyHandler);
}

async function renderResults(arena) {
  const panel = document.getElementById('tab-results');
  panel.replaceChildren(document.getElementById('tpl-results').content.cloneNode(true));

  const report = await api(`/arenas/${arena.id}/results`);
  const o = report.overall;
  const bar = document.getElementById('overall-bar');
  const decided = o.a_wins + o.b_wins;
  const aPct = decided ? (o.a_wins / decided * 100) : 0;
  bar.innerHTML = `<div class="bar-a" style="width:${aPct}%"></div><div class="bar-b"></div>`;
  document.getElementById('overall-summary').textContent =
    `A: ${o.a_wins}  B: ${o.b_wins}  Tie: ${o.ties}  ·  A win rate ${(o.a_win_rate*100).toFixed(1)}% (95% CI ${(o.wilson_low*100).toFixed(1)}–${(o.wilson_high*100).toFixed(1)}%)  ·  disagreement on ${o.disagreement}/${o.total}`;

  const tbody = document.querySelector('#category-table tbody');
  (report.by_category || []).forEach(c => {
    const tr = document.createElement('tr');
    tr.innerHTML = `<td>${escapeHtml(c.label)}</td><td>${(c.a_win_rate*100).toFixed(1)}%</td><td>${c.a_wins}</td><td>${c.b_wins}</td><td>${c.ties}</td><td>${(c.wilson_low*100).toFixed(1)}–${(c.wilson_high*100).toFixed(1)}%</td>`;
    tbody.appendChild(tr);
  });

  if (arena.mode === 'tournament') {
    const elo = await api(`/arenas/${arena.id}/elo`);
    const eloCard = document.getElementById('elo-card');
    eloCard.style.display = '';
    const eloBody = document.querySelector('#elo-table tbody');
    elo.forEach((r, i) => {
      const tr = document.createElement('tr');
      tr.innerHTML = `<td>${i+1}</td><td>${escapeHtml(r.source_label)}</td><td>${Math.round(r.rating)}</td><td>${r.wins}</td><td>${r.losses}</td><td>${r.ties}</td>`;
      eloBody.appendChild(tr);
    });
  }

  document.getElementById('report-link').href = `/api/arenas/${arena.id}/report`;
}

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
}

renderArenaList();
