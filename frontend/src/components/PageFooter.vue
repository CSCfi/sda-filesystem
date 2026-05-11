<script lang="ts" setup>
import { ref, onMounted } from "vue";
import { GetVersion } from "../../wailsjs/go/main/App";

const binaryVersion = ref("xxxx.xx.xx");

onMounted(() => {
  GetVersion().then((version: string) => {
    if (version != "") {
      binaryVersion.value = version;
    }
  });
});
</script>

<template>
  <footer>
    <p class="title">
      Data Gateway – Services for Sensitive Data
    </p>
    <p>CSC – IT Center for Science Ltd.</p>
    <p>Version: {{ binaryVersion }}</p>
  </footer>
</template>

<style scoped>
footer {
  padding: 15px 60px;
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  background-color: var(--c-tertiary-200);
  color: var(--c-text-dark);
}

p {
  font-size: 0.875rem;
}

</style>
