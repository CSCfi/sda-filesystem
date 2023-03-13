<script lang="ts" setup>
import { ref, watch } from 'vue'
import { Login } from '../../wailsjs/go/main/App'
import { EventsEmit } from '../../wailsjs/runtime'

const props = defineProps<{
    initialized: boolean,
    disabled: boolean,
    canSkip: boolean,
}>()

const username = ref("") 
const password = ref("")
const loading = ref(false)
const error401 = ref(false)

watch(() => props.initialized && loading.value, (ready: boolean) => { 
    if (ready) {
        Login(username.value, password.value).then((result: boolean) => {
            if (result) {
                EventsEmit("loggedIn");
            } else {
                error401.value = true;
                loading.value = false;
            }
        }).catch(e => {
            loading.value = false;
            EventsEmit("showToast", "Login error", e as string);
        });
    }
})

function login() {
    if (!props.disabled) {
        loading.value = true;
        error401.value = false;
    }
}

function skip() {
    if (!props.disabled) {
        EventsEmit("loggedIn");
    }
}
</script>

<template>
    <c-container>
        <form id="login-form" v-on:submit.prevent v-on:keyup.enter="login">
            <c-login-card-title>Log in to Data Gateway</c-login-card-title>
            <p>Data Gateway gives you secure access to your data.</p>
            <p class="smaller-text">Please log in with your CSC credentials.</p>
            <c-text-field label="Username" :disabled="props.disabled" v-model="username"></c-text-field>
            <c-text-field label="Password" :disabled="props.disabled" v-model="password" type="password"></c-text-field>

            <c-alert type="error" v-if="error401">
                <div slot="title">Username or password is incorrect</div>
                If you have forgotten your password, visit https://my.csc.fi/forgot-password.
            </c-alert>
         
            <c-row justify="end" gap="20px">
                <c-button
                    size="large" 
                    :disabled="!canSkip"
                    outlined
                    @click="skip">
                    Skip
                </c-button>
                <c-button 
                    size="large" 
                    :loading="loading" 
                    :disabled="props.disabled || !username || !password"
                    @click="login">
                    Login
                </c-button>
            </c-row>
        </form>
    </c-container>
</template>

<style scoped>
#login-form {
    width: 500px;
    display: flex;
    flex-direction: column;
}

#login-form > c-button {
    margin-top: 10px;
}

c-alert > div {
    font-size: 16px;
}

c-alert {
    margin-bottom: 10px;
}
</style>