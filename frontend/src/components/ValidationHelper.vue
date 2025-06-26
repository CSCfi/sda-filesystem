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
    <span class="smaller-text">{{ props.message }}</span>
  </c-row>
</template>
