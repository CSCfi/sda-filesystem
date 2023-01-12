<script lang="ts" setup>
import { reactive, ref, onMounted, computed } from 'vue'
import { GetDefaultMountPoint, LoadFuse, ChangeMountPoint } from '../../wailsjs/go/main/App'
import { CDataTableHeader, CDataTableData, CDataTableFooterOptions, CPaginationOptions } from 'csc-ui/dist/types';
import { EventsEmit, EventsOn } from '../../wailsjs/runtime'
import { filesystem } from "../../wailsjs/go/models";

const pageIdx = ref(1)
const mountpoint = ref("")
const allContainers = ref(0)
const loadedContainers = ref(0)
const globalProgress = computed(() => (allContainers.value <= 0 ? 0 : Math.floor(loadedContainers.value / allContainers.value * 100)))

const projectHeaders: CDataTableHeader[] = [
    { key: 'name', value: 'Name' },
    { key: 'repository', value: 'Location', width: "100px" },
    { 
        key: 'progress', 
        value: 'Progress', 
        width: "300px",
        component: {
            tag: 'c-progress-bar',
            injectValue: true,
            params: {
                style: { width: '100%' },
                singleLine: true,
            },
        },
    },
]

const projectData = reactive<CDataTableData[]>([])
const projectKey = ref(0)

const footerOptions: CDataTableFooterOptions = {
    itemsPerPageOptions: [5, 10, 15, 20],
}
const paginationOptions: CPaginationOptions = {
    itemCount: projectData.length,
    itemsPerPage: 5,
    currentPage: 1,
    startFrom: 0,
    endTo: 4,
}

EventsOn('sendProjects', function(projects: filesystem.Project[]) {
    let tableData: CDataTableData[] = projects.map(project => {
        let item: CDataTableData = Object.fromEntries(Object.entries(project).map(([k, v]) => [k, {"value": v}]));
        item['progress'] = {"value": 0};
        return item;
    });
    projectData.push(...tableData);
    projectKey.value++;
})

EventsOn('showProgress', () => (allContainers.value *= -1))

EventsOn('updateGlobalProgress', function(nom: number, denom: number) {
    loadedContainers.value += nom;
    allContainers.value += denom;
})

EventsOn('updateProjectProgress', function(project: filesystem.Project, progress: number) {
    let idx: number = projectData.findIndex(row => row['name'].value === project.name && row['repository'].value === project.repository);
    projectData[idx]['progress'].value = Math.floor(progress * 100);
    projectKey.value++;
})

onMounted(() => {
    GetDefaultMountPoint().then((dir: string) => {
        mountpoint.value = dir;
    })
})

function changeMountPoint() {
    ChangeMountPoint().then((dir: string) => {
        mountpoint.value = dir;
    }).catch(e => {
        EventsEmit("showToast", "Could not change directory", e as string);
    });
}
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
                <c-text-field id="choose-dir-input" :value="mountpoint" readonly></c-text-field>
                <c-button @click="changeMountPoint" outlined>Change</c-button>
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
            <h2>Preparing access</h2>
            <p class="smaller-text">Please wait, this might take a few minutes.</p>
            <c-progress-bar label="complete" :value=globalProgress></c-progress-bar>
            <c-data-table 
                class="gateway-table"
                :data.prop="projectData" 
                :headers.prop="projectHeaders"
                :key="projectKey" 
                :footerOptions="footerOptions"
                :pagination="paginationOptions"
            ></c-data-table>
        </c-flex>
    </c-container>
</template>

<style>
#choose-dir-input {
    width: 400px;
}
</style>