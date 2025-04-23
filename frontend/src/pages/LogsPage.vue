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

EventsOn('saveLogsAndQuit', function() {
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
      <c-button
        id="export-button"
        text
        no-radius
        @click="SaveLogs(logData)"
      >
        Export detailed logs
        <c-icon :path="mdiTrayArrowDown" />
      </c-button>
    </c-row>
    <c-text-field v-model="filterStr" label="Filter items">
      <c-icon slot="pre" :path="mdiFilterVariant" size="16" />
    </c-text-field>
    <c-data-table
      id="log-table"
      :key="logsKey"
      class="gateway-table"
      no-data-text="No logs available"
      sort-by="timestamp"
      sort-direction="desc"
      :data.prop="logDataTableFiltered"
      :headers.prop="logHeaders"
      :pagination.prop="paginationOptions"
      :hide-footer="logDataTable.length <= 5"
    />
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
