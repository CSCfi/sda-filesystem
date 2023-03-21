<script lang="ts" setup>
import { ref } from 'vue'
import { EventsEmit, EventsOn } from '../../wailsjs/runtime/runtime'

import RepositorySelect from '../components/RepositorySelect.vue'

const props = defineProps<{
    initialized: boolean,
    disabled: boolean,
}>()

const repositories = ref<{[key: string]: [boolean, boolean]}>({})
const repositorySelected = ref<boolean>(false)

EventsOn('setRepositories', function(reps: {[key: string]: [boolean, boolean]}) {
    repositories.value = reps;
})
</script>

<template>
    <c-container style="width: 700px;">
        <c-login-card-title>Log in to Data Gateway</c-login-card-title>
        <p style="width: 500px;">
            Data Gateway gives you secure access to your data folders.
            Please select the service you would like to access data from. 
        </p>

        <c-loader :hide="initialized"></c-loader>
        <RepositorySelect 
            v-for="([repositoryDisabled, useForm], rep) in repositories" 
            @selected="repositorySelected = true"
            :disabled="props.disabled || repositoryDisabled"
            :initialized="props.initialized" 
            :repository="(rep as string)"
            :useForm="useForm">
        </RepositorySelect>
        
        <c-button 
            class="continue-button" 
            :disabled="props.disabled || !repositorySelected"
            @click="EventsEmit('loggedIn')">
            Continue
        </c-button>
    </c-container>
</template>
 