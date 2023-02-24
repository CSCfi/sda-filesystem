<script lang="ts" setup>
import { ref, watch } from 'vue'
import { EventsOn, EventsEmit } from '../../wailsjs/runtime'
import { CAutocompleteItem, CDataTableHeader, CDataTableData } from 'csc-ui/dist/types';
import { SelectFile, CheckEncryption, ExportFile } from '../../wailsjs/go/main/App'
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
                onClick: ({ data }) =>
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

const file = ref("")
const fileEncrypted = ref("")
const modal = ref(false)
const chooseToContinue = ref(false)

EventsOn('isProjectManager', () => {
    pageIdx.value = 1;
})

EventsOn('setBuckets', (buckets: string[]) => {
    bucketItems.value = buckets.map((bucket: string) => ({
        value: bucket,
        name: bucket,
    }))
    filteredBucketItems.value = bucketItems.value;
})

EventsOn('setExportFilenames', (fileOrig: string, fileEnc: string) => { 
    file.value = fileOrig;
    fileEncrypted.value = fileEnc;
    let exportRow: CDataTableData = {
        'name': {'value': fileEnc.split('/').reverse()[0]}, 
        'folder': {'value': selectedBucket.value}
    };
    exportData.value = [];
    exportData.value.push(exportRow);
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
        CheckEncryption(filename, selectedBucket.value).then((exists: boolean) => {
            if (exists) {
                modal.value = true;
            } else {
                chooseToContinue.value = true;
            }
        }).catch(e => {
            EventsEmit("showToast", "Failed to check if file is encrypted", e as string);
        })
    }).catch(e => {
        EventsEmit("showToast", "Could not choose file", e as string);
    });
}

function exportFile() {
    ExportFile(selectedBucket.value, file.value, fileEncrypted.value).then(() => {
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
    <c-container class="fill-width">
        <c-steps :value="pageIdx" :style="{display: pageIdx ? 'block' : 'none'}">
            <c-step>Choose directory</c-step>
            <c-step>Export files</c-step>
            <c-step>Export complete</c-step>
        </c-steps>

        <c-flex v-show="pageIdx == 0">
            <h2>Export is not possible</h2>
            <p>You need to be project manager to export files.</p>
        </c-flex>
        <c-flex v-show="pageIdx == 1">
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
        </c-flex>
        <c-flex v-show="pageIdx == 2">
            <div
                v-if="!chooseToContinue"
                id="drop-area"
                class="fill-width">
                <c-row align="center" gap="20px">
                    <!-- <h4>Drag or drop file here or</h4> -->
                    <c-button outlined @click="selectFile()">Select file</c-button>
                </c-row>
                <p>If you wish to export multiple files, please create a tar/zip file.</p>
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
        </c-flex>
        <c-flex v-show="pageIdx == 3">
            <h2>Exporting File</h2>
            <p>Please wait, this might take few minutes.</p>
            <c-progress-bar indeterminate></c-progress-bar>
            <c-data-table
                class="gateway-table"
                :data.prop="exportData" 
                :headers.prop="exportHeaders"
                hide-footer=true>
            </c-data-table>
        </c-flex>
        <c-flex v-show="pageIdx == 4">
            <h2>Export complete</h2>
            <p>All files have been uploaded to SD Connect. You can now 
                close or minimise the window to continue working.</p>
            <c-button
                class="continue-button" 
                size="large" 
                @click="exportData.pop(); pageIdx = 1">
                New Export
            </c-button>
        </c-flex>
        <c-modal :modal="modal" v-control>
            <c-card>
                <c-card-title>File already exists</c-card-title>

                <c-card-content>
                    Airlock wants to upload file " + {{ fileEncrypted }} + " but a similar 
                    file already exists in SD Connect. Overwrite file?"  
                </c-card-content>

                <c-card-actions>
                    <c-button @click="modal = false">Cancel</c-button>
                    <c-button @click="modal = false; chooseToContinue = true">Overwrite and Continue</c-button>
                </c-card-actions>
            </c-card>
        </c-modal>
    </c-container>
</template>

<style>
c-autocomplete {
    width: 500px;
}

#drop-area {
    border: 3px solid var(--csc-primary);
    height: 200px;
    margin-top: 20px;
    margin-bottom: 20px;
    display: flex;
    justify-content: center;
    align-items: center;
    flex-direction: column;
}

#export-table {
    margin-bottom: 20px;
}
</style>