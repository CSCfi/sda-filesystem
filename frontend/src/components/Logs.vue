<script lang="ts" setup>
import { SaveLogs } from '../../wailsjs/go/main/LogHandler'
import { EventsOn } from '../../wailsjs/runtime'
import { main } from "../../wailsjs/go/models";
import { CDataTableHeader, CDataTableData, CDataTableDataItem, CDataTableFooterOptions, CPaginationOptions } from 'csc-ui/dist/types';
import { reactive, ref, onUnmounted } from 'vue';

const logHeaders: CDataTableHeader[] = [
    { key: 'loglevel', value: 'Level', sortable: false, width: "120px" },
    { 
        key: 'timestamp', 
        value: 'Date and Time', 
        sortable: false,
        width: "200px",
    },
    { key: 'message', value: 'Message', sortable: false },
]

const logData = reactive<main.Log[]>([])
const logDataTable = reactive<CDataTableData[]>([])
const logsKey = ref(-1)
const interval = ref(setInterval(() => addLogsToTable(), 1000))

const footerOptions: CDataTableFooterOptions = {
    itemsPerPageOptions: [5, 10, 15, 20],
}
const paginationOptions: CPaginationOptions = {
    itemCount: logDataTable.length,
    itemsPerPage: 5,
    currentPage: 1,
    startFrom: 0,
    endTo: 4,
}

onUnmounted(() => {
    clearInterval(interval.value);
})

EventsOn('newLogEntry', function(entry: main.Log) {
    logData.push(entry);
})

function addLogsToTable() {
    if (logsKey.value === -1) {
        logsKey.value = 0;
        return;
    }
    if (logsKey.value >= logData.length) {
        return;
    }

    let tableData: CDataTableData[] = logData.map((logRow: main.Log) => {
        //Object.fromEntries(Object.entries(log).map(([k, v]) => [k, {"value": v}]));
        let timestamp: CDataTableDataItem = {
            "value": logRow.timestamp, 
            "formattedValue": logRow.timestamp.split(".")[0],
        };
        let level: CDataTableDataItem = {
            "component": {
                tag: 'c-status', 
                params: { type: logRow.loglevel },
            },
            "value": logRow.loglevel.charAt(0).toUpperCase() + logRow.loglevel.slice(1)
        };
        let message: CDataTableDataItem = {"value": logRow.message[0]};
        return {'loglevel': level, 'timestamp': timestamp, 'message': message};
    });

    logDataTable.push(...tableData);
    logsKey.value += tableData.length;
}
</script>

<template>
    <c-container class="fill-width">
        <c-row justify="space-between" align="center">
            <h2 id="log-title">Logs</h2>
            <c-button text no-radius @click="SaveLogs(logData)">
                <i class="material-icons" slot="icon">logout</i>
                Export detailed logs
            </c-button>
        </c-row>
        <c-data-table 
            class="gateway-table"
            no-data-text="No logs available" 
            sortBy="timestamp"
            :key="logsKey" 
            :data.prop="logDataTable" 
            :headers.prop="logHeaders"
            :footerOptions="footerOptions"
            :pagination="paginationOptions">
        </c-data-table>
    </c-container>
</template>

<style>
#log-title {
    margin-bottom: 0px;
}
</style>