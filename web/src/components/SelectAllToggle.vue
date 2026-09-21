<script setup lang="ts">
// The "select all" control of a list: the same checkbox as the one on each row, with its label. `indeterminate` is a
// partial selection.
defineProps<{ checked: boolean; label: string; indeterminate?: boolean }>()
const emit = defineEmits<{ toggle: [] }>()
</script>

<template>
  <label class="flex items-center gap-2 cursor-pointer hover:text-slate-200 transition-colors">
    <input
      type="checkbox"
      :checked="checked"
      :indeterminate="indeterminate"
      @change="emit('toggle')"
      class="select-box"
    />
    <span>{{ label }}</span>
  </label>
</template>
