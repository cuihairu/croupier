export default async function register({ registerRenderer }) {
  // ensure echarts present (reuse CDN)
  async function ensureEcharts() {
    if (window.echarts) return window.echarts;
    await new Promise((resolve, reject) => {
      const s = document.createElement('script');
      s.src = 'https://cdn.jsdelivr.net/npm/echarts@5/dist/echarts.min.js';
      s.async = true; s.onload = resolve; s.onerror = reject; document.head.appendChild(s);
    });
    return window.echarts;
  }
  // simple bar chart renderer: echarts.bar
  registerRenderer('echarts.bar', (props) => {
    const React = window.React || (await import('react'));
    const { useEffect, useRef, useState } = React;
    const { data, options } = props;
    const ref = useRef(null);
    const [ready, setReady] = useState(false);
    useEffect(() => {
      let disposed = false;
      ensureEcharts().then((echarts) => {
        if (disposed) return; setReady(true);
        const el = ref.current; if (!el) return;
        const inst = echarts.init(el);
        const categories = Array.isArray(data) ? data.map((d)=> d.name || '') : [];
        const values = Array.isArray(data) ? data.map((d)=> Number(d.value)||0) : [];
        inst.setOption({
          tooltip: {}, xAxis: { type: 'category', data: categories }, yAxis: { type: 'value' },
          series: [{ type: 'bar', data: values }], ...options
        });
        const onResize = () => inst.resize(); window.addEventListener('resize', onResize);
        return () => { try { window.removeEventListener('resize', onResize); inst.dispose(); } catch(e){} };
      });
      return () => { disposed = true; };
    }, [JSON.stringify(data), JSON.stringify(options)]);
    return React.createElement('div', { ref, style: { width: '100%', height: 320, border: '1px solid #f0f0f0' } }, !ready && 'Loading chart...');
  });
}

