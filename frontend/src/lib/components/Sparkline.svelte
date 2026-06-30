<script>
  let { data = [], color = 'var(--color-primary)', height = 80, fill = false } = $props();

  function pathFor(values, w, h) {
    if (!values.length) return '';
    const max = Math.max(...values, 1);
    const step = w / (values.length - 1 || 1);
    let d = '';
    values.forEach((v, i) => {
      const x = i * step;
      const y = h - (v / max) * h;
      d += `${i === 0 ? 'M' : 'L'} ${x} ${y}`;
    });
    return d;
  }

  function areaPath(values, w, h) {
    if (!values.length) return '';
    const line = pathFor(values, w, h);
    if (!line) return '';
    return `${line} L ${w} ${h} L 0 ${h} Z`;
  }
</script>

<svg class="w-full" style="height: {height}px" preserveAspectRatio="none">
  {#if fill}
    <path d={areaPath(data, 300, height)} fill="{color}" opacity="0.15" />
  {/if}
  <path d={pathFor(data, 300, height)} fill="none" stroke="{color}" stroke-width="2" vector-effect="non-scaling-stroke" />
</svg>
