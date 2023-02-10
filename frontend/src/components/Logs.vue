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
const logsKey = ref(0)

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

EventsOn('newLogEntry', function(entry: main.Log) {
    logData.push(entry);

    let timestamp: CDataTableDataItem = {
        "value": entry.timestamp, 
        "formattedValue": entry.timestamp.split(".")[0],
    };
    let level: CDataTableDataItem = {
        "component": {
            tag: 'c-status', 
            params: { type: entry.loglevel },
        },
        "value": entry.loglevel.charAt(0).toUpperCase() + entry.loglevel.slice(1)
    };
    let message: CDataTableDataItem = {"value": entry.message[0]};

    let logRow: CDataTableData = {'loglevel': level, 'timestamp': timestamp, 'message': message}
    logDataTable.push(logRow);
})
</script>

<template>
    <c-container class="fill-width">
        <c-row id="log-title-row" justify="space-between" align="center">
            <h2 id="log-title">Logs</h2>
            <c-button id="export-button" text no-radius @click="SaveLogs(logData)">
                <i class="material-icons" slot="icon">logout</i>
                Export detailed logs
            </c-button>
        </c-row>
        <c-text-field
            label="Filter items">
            <i class="material-icons" slot="pre">filter_list</i>
        </c-text-field>
        <c-data-table 
            id="log-table"
            class="gateway-table"
            no-data-text="No logs available" 
            sortBy="timestamp"
            :key="logsKey" 
            :data.prop="logDataTable" 
            :headers.prop="logHeaders"
            :footerOptions="footerOptions"
            :pagination="paginationOptions"
            :hide-footer="logDataTable.length <= 5">
        </c-data-table>
    </c-container>
</template>

<style>
#log-title-row {
    display: block;
    margin-bottom: 20px;
}

#log-title {
    margin-bottom: 0px;
}

#log-table {
    margin-top: 0px;
}
</style>