<script lang="ts" setup>
import { mdiInformation, mdiCheckCircle, mdiCloseCircle} from "@mdi/js";
import { ValidationIconType } from "../types/common";

const props = defineProps<{
  type: ValidationIconType,
  message: string,
}>();

</script>

<template>
  <c-row
    align="center"
    gap="10"
    nowrap
  >
    <c-icon
      :key="props.type"
      :path="props.type === 'success' ?
        mdiCheckCircle : props.type === 'error' ?
          mdiCloseCircle : mdiInformation"
      :style="`color: var(--c-${props.type}-600)`"
    />
    <span>{{ props.message }}</span>
  </c-row>
</template>

<style scoped>
c-row {
  margin-top: 5px;
}
span {
  font-size: 0.9rem;
}
</style>
