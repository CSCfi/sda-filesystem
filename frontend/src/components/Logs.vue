<script lang="ts" setup>
import { SaveLogs } from '../../wailsjs/go/main/LogHandler'
import { EventsOn } from '../../wailsjs/runtime'
import { main } from "../../wailsjs/go/models";
import { CDataTableHeader, CDataTableData, CDataTableFooterOptions } from 'csc-ui/dist/types';
import { reactive, ref } from 'vue';

const levelHeader: string = 'loglevel'
const logHeaders: CDataTableHeader[] = [
    { key: levelHeader, value: 'Level', sortable: false, width: "100px" },
    { 
        key: 'timestamp', 
        value: 'Date and Time', 
        sortable: false, 
        align: "start", 
        width: "200px",
    },
    { key: 'message', value: 'Message', sortable: false, align: "start" },
]

const logData = reactive<CDataTableData[]>([])

const logsKey = ref(0);
const footerOptions: CDataTableFooterOptions = {
    itemsPerPageOptions: [5, 10, 15, 20],
};

EventsOn('newLogEntry', function(entry: main.Log) {
    console.log(entry);
    let item: CDataTableData = Object.fromEntries(Object.entries(entry).map(([k, v]) => [k, {"value": v}]));
    if (item[levelHeader]) {
        let level: string = item[levelHeader].value as string;
        item[levelHeader].component = {
            tag: 'c-status', 
            params: { type: level },
        };
        item[levelHeader].value = level.charAt(0).toUpperCase() + level.slice(1);
    }
    logData.push(item);
    logsKey.value++;
})

function saveLogs() {
    // filedialog
    // SaveLogs()
    console.log("saving logs")
}
</script>

<template>
    <c-container class="fill-width">
        <c-row justify="space-between" align="center">
            <h2>Logs</h2>
            <c-button text no-radius @click="saveLogs">
                <i class="material-icons" slot="icon">logout</i>
                Export detailed logs
            </c-button>
        </c-row>
        <c-data-table 
            class="gateway-table"
            no-data-text="No logs available" 
            :key="logsKey" 
            :data.prop="logData" 
            :headers.prop="logHeaders"
            :footerOptions="footerOptions">
        </c-data-table>
    </c-container>
</template>

<style></style>