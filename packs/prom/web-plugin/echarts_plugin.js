export default function register({ registerRenderer, React }) {
  if (!registerRenderer || !React) return;

  async function ensureEcharts() {
    if (window.echarts) return window.echarts;
    await new Promise((resolve, reject) => {
      const s = document.createElement('script');
      s.src = 'https://cdn.jsdelivr.net/npm/echarts@5/dist/echarts.min.js';
      s.async = true;
      s.onload = resolve;
      s.onerror = reject;
      document.head.appendChild(s);
    });
    return window.echarts;
  }

  const EchartsBar = ({ data, options }) => {
    const { useEffect, useRef, useState } = React;
    const ref = useRef(null);
    const [ready, setReady] = useState(false);

    useEffect(() => {
      let disposed = false;
      let inst = null;
      let onResize = null;

      ensureEcharts()
        .then((echarts) => {
          if (disposed) return;
          setReady(true);

          const el = ref.current;
          if (!el) return;

          inst = echarts.init(el);
          const categories = Array.isArray(data) ? data.map((d) => d?.name || '') : [];
          const values = Array.isArray(data) ? data.map((d) => Number(d?.value) || 0) : [];
          inst.setOption({
            tooltip: {},
            xAxis: { type: 'category', data: categories },
            yAxis: { type: 'value' },
            series: [{ type: 'bar', data: values }],
            ...(options || {}),
          });

          onResize = () => inst && inst.resize();
          window.addEventListener('resize', onResize);
        })
        .catch(() => {});

      return () => {
        disposed = true;
        try {
          if (onResize) window.removeEventListener('resize', onResize);
        } catch {}
        try {
          if (inst) inst.dispose();
        } catch {}
      };
    }, [JSON.stringify(data), JSON.stringify(options)]);

    return React.createElement(
      'div',
      { ref, style: { width: '100%', height: 320, border: '1px solid #f0f0f0' } },
      !ready ? 'Loading chart...' : null,
    );
  };

  registerRenderer('echarts.bar', (props) => React.createElement(EchartsBar, props));
}
