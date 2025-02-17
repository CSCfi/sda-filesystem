<script lang="ts" setup>
import { ref, watch } from 'vue'
import { Authenticate } from '../../wailsjs/go/main/App'
import { EventsEmit } from '../../wailsjs/runtime/runtime'

import LoginForm from './LoginForm.vue';

const props = defineProps<{
    disabled: boolean,
    useForm: boolean,
    repository: string,
}>()

const emit = defineEmits(['selected'])

const selected = ref(false)
const loading = ref(false)

watch(() => selected.value, (sel: boolean) => { 
    loading.value = sel;
    if (sel && !props.useForm) {
        Authenticate(props.repository).then(() => {
            success();
        }).catch(e => {
            selected.value = false;
            EventsEmit("showToast", "Access refused", e as string);
        }).finally(() => {
            loading.value = false;
        });
    }
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

function success() {
    loading.value = false;
    emit("selected");
}
</script>

<template>
    <c-row align="start">
        <c-row align="center" class="switch-row">
            <c-switch 
                :value="selected"
                :style="{'pointer-events': (selected && !useForm) ? 'none' : 'auto'}"
                :disabled="props.disabled"
                @changeValue="selected = $event.target.value">
            </c-switch>
            <c-loader :hide="!loading || useForm"></c-loader>
            <div class="repository-name">
                <span><b>{{ props.repository.replace("-", " ") }}</b></span>
                <span>{{ getRepoDescription(props.repository) }}</span>
            </div>
        </c-row>
        <div>
            <c-switch :style="{ visibility: 'hidden' }"></c-switch>
            <LoginForm 
                class="login-form"
                v-if="useForm && selected && loading" 
                @loggedIn="success()"
                small>
            </LoginForm>
        </div>
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