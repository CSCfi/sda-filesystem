<script lang="ts" setup>
import { DeleteProjects } from '../../wailsjs/go/main/ProjectHandler'
import { reactive, ref, onMounted, computed } from 'vue'
import {
  GetDefaultMountPoint,
  OpenFuse,
  RefreshFuse,
  ChangeMountPoint,
  FilesOpen,
  InitFuse,
} from '../../wailsjs/go/main/App'
import {
  CDataTableHeader,
  CDataTableData,
  CPaginationOptions
} from '@cscfi/csc-ui/dist/types';
import { EventsEmit, EventsOn } from '../../wailsjs/runtime'

const projectHeaders: CDataTableHeader[] = [
  { key: 'name', value: 'Name' },
  { key: 'repository', value: 'Location' },
  {
    key: 'progress',
    value: 'Progress',
    width: "200px",
    sortable: false,
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

const pageIdx = ref(1)
const projectKey = ref(0)
const updating = ref(false)
const mountpoint = ref("")
const loading = ref(false)

const allContainers = ref(0)
const loadedContainers = ref(0)
const globalProgress = computed(() => (allContainers.value <= 0 ? 0 : Math.floor(loadedContainers.value / allContainers.value * 100)))

const paginationOptions: CPaginationOptions = {
  itemCount: projectData.length,
  itemsPerPage: 5,
  currentPage: 1,
  startFrom: 0,
  endTo: 4,
}

onMounted(() => {
  GetDefaultMountPoint().then((dir: string) => {
    mountpoint.value = dir;
  })
  DeleteProjects(); // so that reloading in development mode does not duplicate data
})

EventsOn('showProgress', () => {
  pageIdx.value = 2;
  loading.value = false;
  allContainers.value *= -1;
})

EventsOn('updateGlobalProgress', (nom: number, denom: number) => {
  loadedContainers.value += nom;
  allContainers.value += denom;
})

EventsOn('updateProjectProgress', (name: string, repository: string, progress: number) => {
  let idx: number = projectData.findIndex(row => row['name'].value === name && row['repository'].value === repository);
  if (idx == -1) {
    let item: CDataTableData = {name: {value: name}, repository: {value: repository}};
    item['repository'].formattedValue = repository.replace("-", " ");
    item['progress'] = {"value": 0};
    projectData.push(item);
  } else if (progress > (projectData[idx]['progress'].value as number)) {
    projectData[idx]['progress'].value = progress;
  }
  projectKey.value++;
})

EventsOn('fuseReady', () => {pageIdx.value = 4; updating.value = false})

EventsOn('refresh', () => refresh())

function changeMountPoint() {
  ChangeMountPoint().then((dir: string) => {
    mountpoint.value = dir;
  }).catch(e => {
    EventsEmit("showToast", "Could not change directory", e as string);
  });
}

function refresh() {
  updating.value = true;

  FilesOpen().then((open: boolean) => {
    if (open) {
      EventsEmit(
        "showToast",
        "Refresh not possible",
        "You have files in use which prevents updating Data Gateway"
      )
      updating.value = false;
    } else {
      allContainers.value = 0;
      loadedContainers.value = 0;

      projectData.forEach((project) => {
        project['progress'].value = 0;
      });
      projectKey.value = 0;

      RefreshFuse();
    }
  });
}
</script>

<template>
  <div class="container">
    <c-steps :value="pageIdx">
      <c-step>Choose directory</c-step>
      <c-step>Prepare access</c-step>
      <c-step>Access ready</c-step>
    </c-steps>

    <div v-if="pageIdx == 1">
      <h2>Start by creating access to your files</h2>
      <p>Choose in which local directory your files will be available.</p>
      <c-row gap="40">
        <c-text-field :value="mountpoint" hide-details readonly />
        <c-button outlined @click="changeMountPoint">
          Change
        </c-button>
      </c-row>
      <c-button
        class="continue-button"
        size="large"
        :loading="loading"
        :disabled="!mountpoint"
        @click="if (!loading) InitFuse(); loading = true;"
      >
        Continue
      </c-button>
    </div>
    <div v-else>
      <h2>{{ pageIdx == 2 ? "Preparing access" : "Access ready" }}</h2>
      <div v-if="pageIdx > 2">
        <p>If you update the contents of projects, please refresh access.</p>
        <c-row gap="20" justify="end">
          <c-button outlined :disabled="updating" @click="refresh">
            Refresh access
          </c-button>
          <c-button :disabled="updating" @click="OpenFuse">
            Open folder
          </c-button>
        </c-row>
      </div>
      <p class="smaller-text">
        {{ pageIdx == 2 ? "Please wait, this might take a few minutes." : "Data Gateway is ready to be used." }}
      </p>
      <c-progress-bar label="complete" :value="globalProgress" />
      <c-data-table
        :key="projectKey"
        class="gateway-table"
        :data.prop="projectData"
        :headers.prop="projectHeaders"
        :pagination="paginationOptions"
        :hide-footer="projectData.length <= 5"
      />
    </div>
  </div>
</template>

