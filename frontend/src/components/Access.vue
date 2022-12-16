<script lang="ts" setup>
import { ref, onMounted } from 'vue'
import { GetDefaultMountPoint, LoadFuse } from '../../wailsjs/go/main/App'
import { CDataTableHeader, CDataTableData, CDataTableFooterOptions } from 'csc-ui/dist/types';

const pageIdx = ref(1)
const mountpoint = ref("")

const projectHeaders: CDataTableHeader[] = [
    { key: 'name', value: 'Name' },
    { key: 'repository', value: 'Location', align: "end", width: "100px" },
    { 
        key: 'progress', 
        value: 'Progress', 
        align: "end", 
        width: "200px",
        component: {
            tag: 'c-progress-bar',
            injectValue: true,
            params: {
                style: { width: '100%' },
            },
        },
    },
]

onMounted(() => {
    GetDefaultMountPoint().then((dir: string) => {
        mountpoint.value = dir;
    })
})
</script>

<template>
    <c-container class="fill-width">
        <c-steps :value="pageIdx">
            <c-step>Choose directory</c-step>
            <c-step>Prepare access</c-step>
            <c-step>Access ready</c-step>
        </c-steps>

        <c-flex v-if="pageIdx == 1">
            <h2>Start by creating access to your files</h2>
            <p>Choose in which local directory your files will be available.</p>
            <c-row gap="20px">
                <c-text-field id="choose-dir-input" label="Directory" :value="mountpoint" readonly></c-text-field>
                <c-button outlined>Change</c-button>
            </c-row>
            <c-button 
                class="continue-button" 
                size="large" 
                @click="pageIdx++; LoadFuse()"
                :disabled="!mountpoint">
                Continue
            </c-button>
        </c-flex>
        <c-flex v-else>
            <h2>Prepare access</h2>
            <p class="smaller-text">Choose in which local directory your files will be available.</p>
            <c-progress-bar label="complete"></c-progress-bar>
            <c-data-table 
                class="gateway-table"
                :headers.prop="projectHeaders"
            ></c-data-table>
        </c-flex>
    </c-container>
</template>

<style>
#choose-dir-input {
    width: 400px;
}
</style>