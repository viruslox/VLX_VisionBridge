<script>
  export let usage = { cpu: 0, ram: 0, swap: 0 };
</script>

<div class="card">
  <h2 style="text-align: center;">System Usage</h2>
  <div class="metrics">
    <div class="metric left">CPU: {usage.cpu.toFixed(1)}%</div>
    <div class="metric center">RAM: {usage.ram.toFixed(1)}%</div>
    <div class="metric right">Swap: {usage.swap.toFixed(1)}%</div>
  </div>
</div>

<style>
  .card {
    background: #f4f4f4;
    padding: 1rem;
    border-radius: 8px;
    margin-bottom: 1rem;
    text-align: center;
  }
  h2 {
    margin-top: 0;
  }
  .metrics {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .metric {
    flex: 1;
    font-size: 1.1rem;
    font-weight: 500;
  }
  .left {
    text-align: left;
  }
  .center {
    text-align: center;
  }
  .right {
    text-align: right;
  }
  @media (prefers-color-scheme: dark) {
    .card {
      background: #333;
    }
  }
</style>
