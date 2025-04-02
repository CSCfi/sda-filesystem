<script lang="ts" setup>
import { SaveLogs } from '../../wailsjs/go/main/LogHandler'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { main } from "../../wailsjs/go/models";
import { Quit } from '../../wailsjs/go/main/App'
import {
    CDataTableHeader,
    CDataTableData,
    CDataTableDataItem,
    CPaginationOptions,
} from '@cscfi/csc-ui/dist/types';
import { reactive, ref, watch, computed, onUnmounted } from 'vue';
import { mdiFilterVariant, mdiTrayArrowDown } from '@mdi/js';

const logHeaders: CDataTableHeader[] = [
    { key: 'loglevel', value: 'Level', sortable: false },
    { key: 'timestamp', value: 'Date and Time', sortable: false },
    { key: 'message', value: 'Message', sortable: false },
]

const logData = reactive<main.Log[]>([])
const logDataTable = reactive<CDataTableData[]>([])
const logDataTableFiltered = computed(() => logDataTable.filter(row => {
    let matchedLevel: boolean = containsFilterString(row['loglevel'].value as string);
    let matchedStamp: boolean = containsFilterString(row['timestamp'].formattedValue as string);
    let matchedMessage: boolean = containsFilterString(row['message'].value as string);
    return matchedLevel || matchedStamp || matchedMessage;
}))

const logsKey = ref(0)
const filterStr = ref("")
const interval = ref(setInterval(() => addLogsToTable(), 1000))

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

watch(() => logDataTableFiltered.value, () => {
    logsKey.value++;
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
    <div class="container">
        <c-row id="log-title-row" justify="space-between" align="center">
            <h2>Logs</h2>
            <c-button id="export-button" text no-radius @click="SaveLogs(logData)">
                Export detailed logs
                <c-icon :path="mdiTrayArrowDown"></c-icon>
            </c-button>
        </c-row>
        <c-text-field label="Filter items" v-model="filterStr">
            <c-icon :path="mdiFilterVariant" size="16" slot="pre"></c-icon>
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
            :pagination.prop="paginationOptions"
            :hide-footer="logDataTable.length <= 5">
        </c-data-table>
    </div>
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
