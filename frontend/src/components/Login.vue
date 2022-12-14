<script lang="ts" setup>
import { ref, watch } from 'vue'
import { Login } from '../../wailsjs/go/main/App'

const props = defineProps<{
    ready: boolean
}>()

const emit = defineEmits(["proceed"])

const username = ref("") // validity checks?
const password = ref("")
const loading = ref(false)

watch(() => props.ready && loading.value, (ready: boolean) => { 
    if (ready) {
        Login(username.value, password.value).then((result: boolean) => {
            loading.value = false;
            if (result) {
                emit('proceed');
            } else {
                //show 401
            }
        }).catch(e => {
            console.log(e);
        });
    }
})
</script>

<template>
    <c-container>
        <form id="loginForm" v-on:submit.prevent>
            <c-login-card-title>Log in to Data Gateway</c-login-card-title>
            <p>Data Gateway gives you secure access to your data.</p>
            <p id="smaller">Please log in with your CSC credentials.</p>
            <c-text-field label="Username"></c-text-field>
            <c-text-field label="Password" type="password"></c-text-field>

            <c-button size="large" :loading="loading" @click="loading = true">Login</c-button>
        </form>
    </c-container>
</template>

<style>
#loginForm {
    width: 500px;
    display: flex;
    flex-direction: column;
}

#smaller {
    font-size: 14px;
}

#loginForm > c-button {
    margin-top: 10px;
}
</style>