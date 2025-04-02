<script lang="ts" setup>
import { ref, watch } from 'vue'
import { EventsOn, EventsEmit } from '../../wailsjs/runtime/runtime'
import { CAutocompleteItem, CDataTableHeader, CDataTableData } from '@cscfi/csc-ui/dist/types'
import { SelectFile, CheckExistence, ExportFile } from '../../wailsjs/go/main/App'
import { mdiTrashCanOutline } from '@mdi/js'

const exportHeaders: CDataTableHeader[] = [
    { key: 'name', value: 'Name', sortable: false },
    { key: 'folder', value: 'Target Folder', sortable: false },
]

const exportHeadersModifiable: CDataTableHeader[] = [
    { key: 'name', value: 'Name', sortable: false },
    { key: 'folder', value: 'Target Folder', sortable: false },
    { key: 'actions', value: null, sortable: false, justify: "end",
      children: [
        {
            value: 'Remove',
            component: {
            tag: 'c-button',
            params: {
                text: true,
                size: 'small',
                title: 'Remove',
                path: mdiTrashCanOutline,
                onClick: () =>
                    { exportData.value.pop(); chooseToContinue.value = false }
                },
            },
        },
        ],
    },
]

const exportData = ref<CDataTableData[]>([])
const bucketItems = ref<CAutocompleteItem[]>([])
const filteredBucketItems = ref<CAutocompleteItem[]>([])

const pageIdx = ref(0)
const selectedBucket = ref("")
const bucketQuery = ref("")

const selectedFile = ref("")
const showModal = ref(false)
const chooseToContinue = ref(false)

EventsOn('exportPossible', () => {
    pageIdx.value = 1;
})

EventsOn('setBuckets', (buckets: string[]) => {
    bucketItems.value = buckets.map((bucket: string) => ({
        value: bucket,
        name: bucket,
    }))
    filteredBucketItems.value = bucketItems.value;
})

watch(() => bucketQuery.value, (query: string) => {
    selectedBucket.value = query;
    filteredBucketItems.value = bucketItems.value.filter((item: CAutocompleteItem) => {
        if (selectedBucket.value) {
            return containsFilterString(item.name);
        }

        return true;
    })
})

function selectFile() {
    SelectFile().then((filename: string) => {
        CheckExistence(filename, selectedBucket.value).then((found: boolean) => {
            selectedFile.value = filename

            let exportRow: CDataTableData = {
                'name': {'value': filename.split('/').reverse()[0] + ".c4gh"},
                'folder': {'value': selectedBucket.value}
            };
            exportData.value = [];
            exportData.value.push(exportRow);

            if (found) { // If exists
                showModal.value = true;
            } else {
                chooseToContinue.value = true;
            }
        })
    }).catch(e => {
        EventsEmit("showToast", "Could not choose file", e as string);
    });
}

function exportFile() {
    ExportFile(selectedFile.value, selectedBucket.value).then(() => {
        pageIdx.value = 4;
    }).catch(e => {
        pageIdx.value = 2;
        EventsEmit("showToast", "Exporting file failed", e as string);
    })
}

function containsFilterString(str: string): boolean {
    return str.toLowerCase().includes(selectedBucket.value.toLowerCase());
}
</script>

<template>
    <div class="container">
        <c-steps :value="pageIdx" :style="{display: pageIdx ? 'block' : 'none'}">
            <c-step>Choose directory</c-step>
            <c-step>Export files</c-step>
            <c-step>Export complete</c-step>
        </c-steps>

        <c-modal :value="showModal">
            <c-card>
                <c-card-title>File already exists</c-card-title>

                <c-card-content>
                    Airlock wants to upload file {{ exportData.length ? exportData[0]['name']['value'] : "" }}
                    but a similar file already exists in SD Connect. Overwrite file?
                </c-card-content>

                <c-card-actions justify="end">
                    <c-button @click="showModal = false" outlined>Cancel</c-button>
                    <c-button @click="showModal = false; chooseToContinue = true">Overwrite and Continue</c-button>
                </c-card-actions>
            </c-card>
        </c-modal>

        <div v-show="pageIdx == 0" id="no-export-page">
            <h2>Export is not possible</h2>
            <p>You need to have project manager rights to export files.</p>
        </div>
        <div v-show="pageIdx == 1">
            <h2>Select a destination folder for your export</h2>
            <p>
                Your export will be sent to SD Connect.
                If the folder does not already exist in SD Connect, it will be created.
                Please note that the folder name cannot be modified afterwards.
            </p>
            <c-row>
                <c-autocomplete
                    label="Folder name"
                    :items="filteredBucketItems"
                    v-model="selectedBucket"
                    items-per-page=5
                    return-value
                    v-control
                    @changeQuery="bucketQuery = $event.detail">
                </c-autocomplete>
            </c-row>
            <c-button
                class="continue-button"
                size="large"
                :disabled="!selectedBucket"
                @click="pageIdx++">
                Continue
            </c-button>
        </div>
        <div v-show="pageIdx == 2">
            <div
                v-if="!chooseToContinue"
                id="drop-area">
                <c-row align="center" gap="20">
                    <!-- <h4>Drag or drop file here or</h4> -->
                    <c-button outlined @click="selectFile()">Select file</c-button>
                </c-row>
                <p>If you wish to export multiple files, please create a tar/zip file.</p>
                <p>
                    Unencrypted file will be encrypted by default with service encryption key
                    and will be accessible only via SD Desktop.<br>If you want to access the file
                    otherwise, please encrypt it before exporting.
                </p>
            </div>
            <c-data-table v-else
                id="export-table"
                class="gateway-table"
                :data.prop="exportData"
                :headers.prop="exportHeadersModifiable"
                hide-footer=true>
            </c-data-table>
            <c-row justify="space-between">
                <c-button @click="pageIdx--; exportData.pop()" outlined>Cancel</c-button>
                <c-button @click="pageIdx++; exportFile()" :disabled="!chooseToContinue">Export</c-button>
            </c-row>
        </div>
        <div v-show="pageIdx == 3">
            <h2>Exporting File</h2>
            <p>Please wait, this might take few minutes.</p>
            <c-progress-bar indeterminate></c-progress-bar>
            <c-data-table
                class="gateway-table"
                :data.prop="exportData"
                :headers.prop="exportHeaders"
                hide-footer=true>
            </c-data-table>
        </div>
        <div v-show="pageIdx == 4">
            <h2>Export complete</h2>
            <p>
                All files have been uploaded to SD Connect. You can now
                close or minimise the window to continue working.
            </p>
            <c-button
                class="continue-button"
                size="large"
                @click="exportData.pop(); chooseToContinue = false; pageIdx = 1">
                New Export
            </c-button>
        </div>
    </div>
</template>

<style scoped>
c-autocomplete {
    width: 500px;
}

#no-export-page {
    width: 500px;
}

#drop-area {
    border: 3px solid var(--c-primary-600);
    margin-top: 20px;
    margin-bottom: 20px;
    padding: 40px;
    display: flex;
    justify-content: center;
    align-items: center;
    flex-direction: column;
}

#drop-area > p {
    text-align: center;
}

#export-table {
    margin-bottom: 20px;
}
</style>
