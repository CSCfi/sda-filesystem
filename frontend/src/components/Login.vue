<script lang="ts" setup>
import { ref, watch } from 'vue'
import { Login } from '../../wailsjs/go/main/App'
import { EventsEmit } from '../../wailsjs/runtime'

const props = defineProps<{
    ready: boolean,
    disabled: boolean,
}>()

const emit = defineEmits(["proceed"])

const username = ref("") // validity checks?
const password = ref("")
const loading = ref(false)
const error401 = ref(false)

watch(() => props.disabled, (disabled: boolean) => { 
    if (disabled) {
        loading.value = false
    }
})

watch(() => props.ready && loading.value, (ready: boolean) => { 
    if (ready) {
        Login(username.value, password.value).then((result: boolean) => {
            loading.value = false;
            if (result) {
                emit('proceed');
            } else {
                error401.value = true;
            }
        }).catch(e => {
            emit('proceed');
            EventsEmit("showToast", "Login error", e as string);
        });
    }
})
</script>

<template>
    <c-container>
        <form id="login-form" v-on:submit.prevent>
            <c-login-card-title>Log in to Data Gateway</c-login-card-title>
            <p>Data Gateway gives you secure access to your data.</p>
            <p class="smaller-text">Please log in with your CSC credentials.</p>
            <c-text-field label="Username" :disabled="props.disabled"></c-text-field>
            <c-text-field label="Password" type="password" :disabled="props.disabled"></c-text-field>

            <c-alert type="error" v-if="error401">
                <div slot="title">Username or password is incorrect</div>
                If you have forgotten your password, visit https://my.csc.fi/forgot-password.
            </c-alert>
         
            <c-button 
                size="large" 
                :loading="loading" 
                :disabled="props.disabled"
                @click="loading = true; error401 = false;">
                Login
            </c-button>
        </form>
    </c-container>
</template>

<style>
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
</style>