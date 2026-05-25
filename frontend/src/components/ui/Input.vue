<script setup lang="ts">
import { cn } from "@/lib/utils";

withDefaults(
  defineProps<{
    id?: string;
    class?: string;
    type?: string;
    modelValue?: string;
    placeholder?: string;
    required?: boolean;
    minlength?: number;
    maxlength?: number;
    disabled?: boolean;
  }>(),
  {
    type: "text",
    modelValue: "",
    required: false,
    disabled: false,
  },
);

defineEmits<{
  "update:modelValue": [value: string];
}>();
</script>

<template>
  <input
    :id="id"
    :type="type"
    :value="modelValue"
    :placeholder="placeholder"
    :required="required"
    :minlength="minlength"
    :maxlength="maxlength"
    :disabled="disabled"
    :class="
      cn(
        'flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm text-foreground ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50',
        $props.class,
      )
    "
    @input="$emit('update:modelValue', ($event.target as HTMLInputElement).value)"
  />
</template>
