<script lang="ts" setup>
import { ref, watch } from "vue";

const props = defineProps<{
  disabled: boolean,
  repository: string,
}>();

const emit = defineEmits<{
  selected: [status: boolean]
}>();

const selected = ref(false);

watch(() => selected.value, (sel: boolean) => {
  emit("selected", sel);
});

function getRepoDescription(repo: string) {
  if (repo.toLowerCase() === "sd-apply") {
    return ": Access requires a permit from the data controller";
  }
  else if (repo.toLowerCase() === "sd-connect") {
    return ": Access data stored in SD Connect";
  }
  return "";
}
</script>

<template>
  <c-row align="center" gap="10" class="switch-row">
    <c-switch
      v-model="selected"
      v-control
      :disabled="props.disabled"
    />
    <div class="repository-name">
      <span><b>{{ props.repository.replace("-", " ") }}</b></span>
      <span>{{ getRepoDescription(props.repository) }}</span>
    </div>
  </c-row>
</template>

<style scoped>

.repository-name {
  white-space: nowrap;
}
.switch-row {
  margin-top: 1rem;
}
</style>
