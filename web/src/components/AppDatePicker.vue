<script setup lang="ts">
import { computed } from 'vue'
import { VueDatePicker } from '@vuepic/vue-datepicker'
import { fr } from 'date-fns/locale'

const props = withDefaults(
  defineProps<{
    modelValue?: string | Date | null
    enableTimePicker?: boolean
    placeholder?: string
    disabled?: boolean
    required?: boolean
    clearable?: boolean
    id?: string
    size?: 'xs' | 'sm' | 'md'
  }>(),
  {
    modelValue: '',
    enableTimePicker: false,
    placeholder: 'Sélectionner une date',
    disabled: false,
    required: false,
    clearable: undefined,
    id: undefined,
    size: 'sm',
  }
)

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
  (e: 'change', value: string): void
}>()

const internalValue = computed({
  get: () => {
    if (!props.modelValue) return null
    return props.modelValue
  },
  set: (val) => {
    const stringVal = val ? String(val) : ''
    emit('update:modelValue', stringVal)
    emit('change', stringVal)
  },
})

const isClearable = computed(() => {
  if (props.clearable !== undefined) return props.clearable
  return !props.required
})

const modelType = computed(() => {
  return props.enableTimePicker ? "yyyy-MM-dd'T'HH:mm" : 'yyyy-MM-dd'
})

const displayFormat = computed(() => {
  return props.enableTimePicker ? 'dd/MM/yyyy HH:mm' : 'dd/MM/yyyy'
})

const effectivePlaceholder = computed(() => {
  if (props.placeholder && props.placeholder !== 'Sélectionner une date') {
    return props.placeholder
  }
  return props.enableTimePicker ? 'JJ/MM/AAAA HH:mm' : 'JJ/MM/AAAA'
})

const actionRowConfig = computed(() => ({
  showNow: true,
  nowBtnLabel: "Aujourd'hui",
  selectBtnLabel: 'Valider',
  cancelBtnLabel: 'Annuler',
}))
</script>

<template>
  <div :class="['app-datepicker-wrapper', `size-${size}`, { 'is-disabled': disabled }]">
    <VueDatePicker
      :uid="id"
      v-model="internalValue"
      :dark="true"
      :locale="fr"
      :enable-time-picker="enableTimePicker"
      :auto-apply="!enableTimePicker"
      :close-on-auto-apply="!enableTimePicker"
      :model-type="modelType"
      :format="displayFormat"
      :placeholder="effectivePlaceholder"
      :disabled="disabled"
      :clearable="isClearable"
      teleport="body"
      :action-row="actionRowConfig"
    />
  </div>
</template>

<style scoped>
.app-datepicker-wrapper {
  width: 100%;
}

.size-xs :deep(.dp__input) {
  font-size: 0.75rem !important;
  line-height: 1rem !important;
  padding-top: 0.375rem !important;
  padding-bottom: 0.375rem !important;
  padding-left: 1.85rem !important;
  padding-right: 0.375rem !important;
}

.size-xs :deep(.dp__input_icon) {
  padding-left: 0.5rem !important;
}

.size-sm :deep(.dp__input) {
  font-size: 0.875rem !important;
  line-height: 1.25rem !important;
  padding-top: 0.5rem !important;
  padding-bottom: 0.5rem !important;
}

.size-md :deep(.dp__input) {
  font-size: 1rem !important;
  line-height: 1.5rem !important;
  padding-top: 0.625rem !important;
  padding-bottom: 0.625rem !important;
}
</style>
