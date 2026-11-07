<script lang="ts">
  import {
    ArrowRight, ArrowUpRight, ArrowUpDown, BookOpen, Check, ChevronDown,
    ChevronUp, CodeXml, ExternalLink, FileText, Globe, History, Info,
    LockKeyhole, Pencil, Plus, Route, Settings2, ShieldCheck, Trash2,
    UserRound, UsersRound, X, RefreshCw, FlaskConical, Cloud, Laptop,
  } from "@lucide/svelte";
  export let name = "route";
  export let size = 18;
  const icons = {
    route: Route, profiles: UsersRound, test: FlaskConical,
    preferences: Settings2, logs: FileText, info: Info, plus: Plus,
    arrow: ArrowRight, edit: Pencil, up: ChevronUp, down: ChevronDown,
    trash: Trash2, check: Check, close: X, external: ExternalLink,
    globe: Globe, lock: LockKeyhole, code: CodeXml, sync: RefreshCw,
    learning: BookOpen, history: History, cloud: Cloud, laptop: Laptop,
    shield: ShieldCheck, person: UserRound, reorder: ArrowUpDown, launch: ArrowUpRight,
  };
  $: component = icons[name as keyof typeof icons] || Route;
</script>

<svelte:component this={component} {size} strokeWidth={1.75} aria-hidden="true" />
