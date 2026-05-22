<script setup lang="ts">
import { computed } from "vue";

import { cn } from "@/lib/utils";

type BadgeVariant = "default" | "secondary" | "outline";

const variantClasses: Record<BadgeVariant, string> = {
  default: "border-transparent bg-primary text-primary-foreground",
  secondary: "border-transparent bg-secondary text-secondary-foreground",
  outline: "text-foreground",
};

const props = withDefaults(
  defineProps<{
    variant?: BadgeVariant;
    class?: string;
  }>(),
  {
    variant: "default",
  },
);

const classes = computed(() =>
  cn("inline-flex items-center rounded-md border px-2 py-0.5 text-xs font-semibold", variantClasses[props.variant], props.class),
);
</script>

<template>
  <span :class="classes">
    <slot />
  </span>
</template>
