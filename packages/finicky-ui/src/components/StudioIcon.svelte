<script lang="ts">
  export let name = "route";
  export let size = 18;
  const paths: Record<string, string> = {
    history: "M3 11a9 9 0 1 1 2 7M3 4v7h7m2-4v5l4 2",
    route: "M6 3v10a5 5 0 0 0 5 5h7m-4-4 4 4-4 4M6 8h6a5 5 0 0 0 5-5",
    profiles:
      "M5 4h14a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2Zm10 5h3m-3 4h3M6 16c0-4 6-4 6 0M9 8a2 2 0 1 0 0 4 2 2 0 0 0 0-4Z",
    test: "m14 3 7 7-7 7M21 10H8a5 5 0 0 0 0 10h3",
    preferences: "M4 7h16M4 17h16M8 4v6m8 4v6",
    logs: "M5 4h14v16H5ZM8 8h8m-8 4h8m-8 4h5",
    info: "M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18Zm0 8v6m0-10v.1",
    plus: "M12 5v14M5 12h14",
    arrow: "M4 12h16m-6-6 6 6-6 6",
    edit: "m15 4 5 5M4 20l5-1L21 7a2.1 2.1 0 0 0-5-3L4 16Z",
    up: "m6 15 6-6 6 6",
    down: "m6 9 6 6 6-6",
    trash: "M3 6h18M9 6V3h6v3M5 6l1 15h12l1-15M10 10v7m4-7v7",
    check: "m5 12 4 4L19 6",
    close: "m6 6 12 12M6 18 18 6",
    external:
      "M14 3h7v7m0-7L10 14M10 3H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-5",
    globe:
      "M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18ZM3 12h18M12 3a19 19 0 0 1 0 18 19 19 0 0 1 0-18",
    lock: "M5 10h14v11H5ZM8 10V6a4 4 0 0 1 8 0v4m-4 9v-3",
    code: "m8 6-6 6 6 6m8-12 6 6-6 6m-3-15-2 18",
  };
</script>

<svg
  width={size}
  height={size}
  viewBox="0 0 24 24"
  fill="none"
  stroke="currentColor"
  stroke-width="1.65"
  stroke-linecap="round"
  stroke-linejoin="round"
  aria-hidden="true"><path d={paths[name] || paths.route} /></svg
>
