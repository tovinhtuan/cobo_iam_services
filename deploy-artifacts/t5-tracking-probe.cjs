const tok = require('./t5-token-tmp.json').session.access_token;
const ids = [
  '019feb2d-2257-7c09-805e-d028fc2afa6a',
  '019feb2f-0a4c-7c4e-bd84-2d66781151d8',
  '019fec22-6c4a-7abe-ad2c-94d4de6c9fcb',
];
(async () => {
  for (const id of ids) {
    const r = await fetch('http://88.216.208.0:8080/api/v1/company/ad-hoc-proposals/' + id, {
      headers: { Authorization: 'Bearer ' + tok },
    });
    const b = await r.json();
    const t = b.tracking || {};
    const active = (t.steps || []).filter((s) => String(s.status).toUpperCase() === 'ACTIVE');
    const futureActive = (t.steps || []).filter(
      (s) => String(s.status).toUpperCase() === 'ACTIVE' && s.order > (active[0]?.order || 0),
    );
    console.log(
      JSON.stringify(
        {
          id,
          http: r.status,
          status: b.status,
          has_runtime: t.has_runtime,
          completed_steps: t.completed_steps,
          total_steps: t.total_steps,
          activeSteps: active.map((s) => ({
            name: s.name,
            order: s.order,
            assignees: (s.assignees || []).map((a) => a.membership_id || a),
          })),
          futurePresentedAsActive: futureActive.length > 0,
          approval_progress: b.approval_progress,
          deadline_day_type: b.proposed_deadline_day_type,
          proposed_deadline_days: b.proposed_deadline_days,
        },
        null,
        2,
      ),
    );
  }
})();
