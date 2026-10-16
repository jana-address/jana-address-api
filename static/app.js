// Переключатель языка
const lang = document.querySelector('.lang');
lang.querySelector('.lang__btn').addEventListener('click', () => lang.classList.toggle('is-open'));
document.addEventListener('click', e => { if (!lang.contains(e.target)) lang.classList.remove('is-open'); });

// Вкладки «Бизнесу / Партнёрам»
document.querySelectorAll('.tabs__btn').forEach(btn => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('.tabs__btn').forEach(b => b.classList.toggle('is-active', b === btn));
    document.querySelectorAll('.panel').forEach(p => p.classList.toggle('is-active', p.id === 'tab-' + btn.dataset.tab));
  });
});

// Поиск по трек-номеру
document.getElementById('track-form').addEventListener('submit', async e => {
  e.preventDefault();
  const out = document.getElementById('track-result');
  const q = new FormData(e.target).get('q');
  try {
    const res = await fetch('/api/track?q=' + encodeURIComponent(q));
    const data = await res.json();
    out.textContent = data.error || data.status;
  } catch {
    out.textContent = 'Не удалось выполнить поиск. Попробуйте ещё раз.';
  }
});
