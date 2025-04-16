<script lang="ts" setup>
import { ref, watch } from 'vue'

const props = defineProps<{
    disabled: boolean,
    repository: string,
}>()

const emit = defineEmits<{
  selected: [status: boolean]
}>()

const selected = ref(false)

watch(() => selected.value, (sel: boolean) => {
    emit("selected", sel);
})

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
    <c-row align="start">
        <c-row align="center" class="switch-row">
            <c-switch
                :value="selected"
                :disabled="props.disabled"
                @changeValue="selected = $event.target.value">
            </c-switch>
            <div class="repository-name">
                <span><b>{{ props.repository.replace("-", " ") }}</b></span>
                <span>{{ getRepoDescription(props.repository) }}</span>
            </div>
        </c-row>
       <c-switch :style="{ visibility: 'hidden' }"></c-switch>
    </c-row>
</template>

<style scoped>
c-switch {
    padding-bottom: 8px;
    padding-top: 8px;
}

.switch-row {
    position: relative;
}

.repository-name {
    white-space: nowrap;
    position: absolute;
    padding-left: 10px;
    left: 100%;
}

.login-form {
    width: 400px;
    padding-left: 10px;
}
</style>
