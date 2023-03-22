<script lang="ts" setup>
import { ref, watch } from 'vue'
import { Login } from '../../wailsjs/go/main/App'
import { EventsEmit } from '../../wailsjs/runtime/runtime'

const props = defineProps<{
    small?: boolean
}>()

const emit = defineEmits(['loggedIn'])

const loading = ref(false)
const error401 = ref(false)
const username = ref("") 
const password = ref("")

watch(() => loading.value, (ready: boolean) => { 
    if (ready) {
        Login(username.value, password.value).then((result: boolean) => {
            if (result) {
                emit("loggedIn");
            } else {
                error401.value = true;
            }
        }).catch(e => {
            EventsEmit("showToast", "Login error", e as string);
        }).finally(() => {
            loading.value = false;
        });
    }
})

function login() {
    loading.value = true;
    error401.value = false;
}
</script>

<template>
    <form id="login-form" 
        v-on:submit.prevent
        :style="{transform: props.small ? 'translate(-5%, -5%) scale(0.9)' : 'none', gap: props.small ? '10px' : '20px'}">
        <div :class="{'smaller-text': !props.small}">
            Please log in with your CSC credentials.
        </div>
        <c-text-field 
            label="Username" 
            v-model="username" 
            hide-details>
        </c-text-field>
        <c-text-field 
            label="Password" 
            v-model="password" 
            hide-details 
            type="password">
        </c-text-field>

        <c-alert type="error" v-if="error401">
            <div slot="title">Username or password is incorrect</div>
            <div>If you have forgotten your password, visit https://my.csc.fi/forgot-password.</div>
        </c-alert>

        <c-button outlined :loading="loading" @click="login()">
            Login
        </c-button>
    </form>
</template>

<style scoped>
#login-form {
    display: flex;
    flex-direction: column;
    padding-top: 10px;
}

c-button {
    align-self: self-start;
}

c-alert > div {
    font-size: 16px;
}
</style>