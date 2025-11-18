(() => {
  const $ = (id) => document.getElementById(id);
  const api = (path) => path; // same origin

  async function list() {
    const qs = new URLSearchParams();
    const state = $('state').value.trim();
    const fid = $('function_id').value.trim();
    const gid = $('game_id').value.trim();
    const env = $('env').value.trim();
    const page = $('page').value || '1';
    const size = $('size').value || '20';
    if (state) qs.set('state', state);
    if (fid) qs.set('function_id', fid);
    if (gid) qs.set('game_id', gid);
    if (env) qs.set('env', env);
    qs.set('page', page); qs.set('size', size);
    const resp = await fetch(api('/api/approvals?' + qs.toString()), {
      headers: authHdr()
    });
    if (!resp.ok) { alert('list failed: ' + (await resp.text())); return; }
    const data = await resp.json();
    const tbody = $('tbl').querySelector('tbody');
    tbody.innerHTML = '';
    (data.approvals || []).forEach(a => {
      const tr = document.createElement('tr');
      tr.innerHTML = `
        <td>${a.CreatedAt || ''}</td>
        <td>${a.Actor || ''}</td>
        <td>${a.FunctionID || ''}</td>
        <td>${a.GameID || ''}/${a.Env || ''}</td>
        <td>${a.State || ''}</td>
        <td>${a.Mode || ''}</td>
        <td>${a.Route || ''}</td>
        <td class="row-actions">
          <button data-id="${a.ID}" data-act="view">View</button>
          ${a.State === 'pending' ? `<button data-id="${a.ID}" data-act="approve">Approve</button>
          <button data-id="${a.ID}" data-act="reject">Reject</button>` : ''}
        </td>`;
      tbody.appendChild(tr);
    });
  }

  function authHdr() {
    const tok = $('jwt').value.trim();
    return tok ? { 'Authorization': 'Bearer ' + tok } : {};
  }

  async function view(id) {
    const resp = await fetch(api('/api/approvals/get?id=' + encodeURIComponent(id)), { headers: authHdr() });
    if (!resp.ok) { alert('get failed: ' + (await resp.text())); return; }
    const d = await resp.json();
    $('detail').textContent = JSON.stringify(d, null, 2);
    $('preview').textContent = d.payload_preview || '';
  }

  async function approve(id) {
    if (!confirm('Approve ' + id + '?')) return;
    const otp = $('otp').value.trim();
    const resp = await fetch(api('/api/approvals/approve'), {
      method: 'POST', headers: { 'Content-Type': 'application/json', ...authHdr() },
      body: JSON.stringify({ id, otp })
    });
    if (!resp.ok) { alert('approve failed: ' + (await resp.text())); return; }
    alert('approved'); await list(); await view(id);
  }

  async function reject(id) {
    const reason = prompt('Reject reason?') || '';
    const resp = await fetch(api('/api/approvals/reject'), {
      method: 'POST', headers: { 'Content-Type': 'application/json', ...authHdr() },
      body: JSON.stringify({ id, reason })
    });
    if (!resp.ok) { alert('reject failed: ' + (await resp.text())); return; }
    alert('rejected'); await list(); await view(id);
  }

  $('load').addEventListener('click', list);
  $('tbl').addEventListener('click', (e) => {
    const btn = e.target.closest('button'); if (!btn) return;
    const id = btn.getAttribute('data-id');
    const act = btn.getAttribute('data-act');
    if (act === 'view') view(id);
    if (act === 'approve') approve(id);
    if (act === 'reject') reject(id);
  });

  list().catch(console.error);
})();
