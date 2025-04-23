<script lang="ts" setup>
import { ref, computed } from "vue";
import { EventsEmit, EventsOn } from "../../wailsjs/runtime/runtime";

import RepositorySelect from "../components/RepositorySelect.vue";

const props = defineProps<{
  initialized: boolean,
  disabled: boolean,
}>();

const repositories = ref<{[key: string]: boolean}>({});
const selectedRepos = ref<{[key: string]: boolean}>({});
const validSelection = computed(() => Object.values(selectedRepos.value).some(Boolean));

EventsOn("setRepositories", function(reps: {[key: string]: boolean}) {
  repositories.value = reps;
  selectedRepos.value = Object.fromEntries(
    Object.entries(reps)
      .filter(([_, disabled]) => !disabled)
      .map(([key]) => [key, false])
  );
});
</script>

<template>
  <div style="width: 700px;">
    <c-login-card-title>Log in to Data Gateway</c-login-card-title>
    <p style="width: 550px;">
      Access and import files from other SD services into SD Desktop.
      Please select the service to access data from.
    </p>

    <c-loader :hide="initialized || disabled" />
    <RepositorySelect
      v-for="(repositoryDisabled, rep) in repositories"
      :key="rep"
      :disabled="props.disabled || repositoryDisabled"
      :repository="(rep as string)"
      @selected="(status: boolean) => selectedRepos[rep] = status"
    />

    <c-button
      class="continue-button"
      :disabled="props.disabled || !validSelection"
      @click="EventsEmit('selectFinished')"
    >
      Continue
    </c-button>
  </div>
</template>
