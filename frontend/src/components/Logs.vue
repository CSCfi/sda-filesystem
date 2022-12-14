<script lang="ts" setup>
import { SaveLogs } from '../../wailsjs/go/main/LogHandler'
import { EventsOn } from '../../wailsjs/runtime'
import { main } from "../../wailsjs/go/models";
import { CDataTableHeader, CDataTableData } from 'csc-ui/dist/types/types';
import { reactive } from 'vue';

const logHeaders: CDataTableHeader[] = [
    { key: 'loglevel', value: 'Level', sortable: false },
    { key: 'timestamp', value: 'Date and Time', sortable: false },
    { key: 'message', value: 'Message', sortable: false },
]

const logData = reactive<CDataTableData[]>([])

EventsOn('newLogEntry', function(entry?: main.Log) {
    if (entry) {
        let item = Object.fromEntries(Object.entries(entry).map(([k, v]) => [k, {"value": v}]));
        logData.push(item);
    }
})

function saveLogs() {
    // filedialog
    // SaveLogs()
    console.log("saving logs")
}
</script>

<template>
    <c-container class="fillWidth">
        <c-row justify="space-between" align="center">
            <h1>Logs</h1>
            <c-button text no-radius @click="saveLogs()">
                <i class="material-icons" slot="icon">logout</i>
                Export detailed logs
            </c-button>
        </c-row>
        <c-data-table id="logs" no-data-text="No logs available" :data.prop="logData" :headers.prop="logHeaders"></c-data-table>
    </c-container>
</template>

<style>
#logs {
    margin-top: 20px;
}
</style>