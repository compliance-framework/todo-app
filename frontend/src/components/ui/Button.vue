<script setup lang="ts">
import { computed } from "vue";

import { cn } from "@/lib/utils";

type ButtonVariant = "default" | "destructive" | "outline" | "secondary" | "ghost";
type ButtonSize = "default" | "sm" | "icon";

const variantClasses: Record<ButtonVariant, string> = {
  default: "bg-primary text-primary-foreground hover:bg-primary/90",
  destructive: "bg-destructive text-destructive-foreground hover:bg-destructive/90",
  outline: "border border-input bg-background hover:bg-accent hover:text-accent-foreground",
  secondary: "bg-secondary text-secondary-foreground hover:bg-secondary/80",
  ghost: "hover:bg-accent hover:text-accent-foreground",
};

const sizeClasses: Record<ButtonSize, string> = {
  default: "h-10 px-4 py-2",
  sm: "h-9 rounded-md px-3",
  icon: "h-9 w-9",
};

const props = withDefaults(
  defineProps<{
    variant?: ButtonVariant;
    size?: ButtonSize;
    class?: string;
    type?: "button" | "submit" | "reset";
    disabled?: boolean;
  }>(),
  {
    variant: "default",
    size: "default",
    type: "button",
    disabled: false,
  },
);

const classes = computed(() =>
  cn(
    "inline-flex items-center justify-center whitespace-nowrap rounded-md text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50",
    variantClasses[props.variant],
    sizeClasses[props.size],
    props.class,
  ),
);
</script>

<template>
  <button :type="type" :class="classes" :disabled="disabled">
    <slot />
  </button>
</template>
