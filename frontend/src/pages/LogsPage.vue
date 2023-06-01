<script lang="ts" setup>
import { SaveLogs } from '../../wailsjs/go/main/LogHandler'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { main } from "../../wailsjs/go/models";
import { Quit } from '../../wailsjs/go/main/App'
import { CDataTableHeader, CDataTableData, CDataTableDataItem, CDataTableFooterOptions, CPaginationOptions } from 'csc-ui/dist/types';
import { reactive, ref, computed, onUnmounted } from 'vue';

const logHeaders: CDataTableHeader[] = [
    { key: 'loglevel', value: 'Level', sortable: false },
    { key: 'timestamp', value: 'Date and Time', sortable: false },
    { key: 'message', value: 'Message', sortable: false },
]

const logData = reactive<main.Log[]>([])
const logDataTable = reactive<CDataTableData[]>([])
const logDataTableFiltered = computed(() => logDataTable.filter(row => {
    logsKey.value++;
    let matchedLevel: boolean = containsFilterString(row['loglevel'].value as string);
    let matchedStamp: boolean = containsFilterString(row['timestamp'].formattedValue as string);
    let matchedMessage: boolean = containsFilterString(row['message'].value as string);
    return matchedLevel || matchedStamp || matchedMessage;
}))

const logsKey = ref(0)
const filterStr = ref("") 
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

EventsOn('saveLogsAndQuit', function(entry: main.Log) {
    SaveLogs(logData).then(() => Quit());
})

function addLogsToTable() {
    if (logData.length <= logDataTable.length) {
        return;
    }

    let tableData: CDataTableData[] = logData.slice(logDataTable.length).map((logRow: main.Log) => {
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
}

function containsFilterString(str: string): boolean {
    return str.toLowerCase().includes(filterStr.value.toLowerCase());
}
</script>

<template>
    <c-container class="fill-width">
        <c-row id="log-title-row" justify="space-between" align="center">
            <h2>Logs</h2>
            <c-button id="export-button" text no-radius @click="SaveLogs(logData)">
                <i class="mdi mdi-tray-arrow-down" slot="icon"></i>
                Export detailed logs
            </c-button>
        </c-row>
        <c-text-field label="Filter items" v-model="filterStr">
            <i class="mdi mdi-filter-variant" slot="pre"></i>
        </c-text-field>
        <c-data-table 
            id="log-table"
            class="gateway-table"
            no-data-text="No logs available" 
            sortBy="timestamp"
            sort-direction="desc"
            :key="logsKey" 
            :data.prop="logDataTableFiltered" 
            :headers.prop="logHeaders"
            :footerOptions.prop="footerOptions"
            :pagination.prop="paginationOptions"
            :hide-footer="logDataTable.length <= 5">
        </c-data-table>
    </c-container>
</template>

<style scoped>
#log-title-row {
    display: block;
    margin-bottom: 20px;
}

#log-title-row > h2 {
    margin-bottom: 0px;
}

#log-table {
    margin-top: 0px;
}
</style>