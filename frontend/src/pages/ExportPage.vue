<script lang="ts" setup>
import { ref, watch } from 'vue'
import { EventsOn, EventsEmit } from '../../wailsjs/runtime/runtime'
import { CAutocompleteItem, CDataTableHeader, CDataTableData } from 'csc-ui/dist/types';
import { SelectFile, CheckEncryption, ExportFile } from '../../wailsjs/go/main/App'
import { mdiTrashCanOutline } from '@mdi/js'

import LoginForm from '../components/LoginForm.vue';

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

const skipLogin = ref(false)
const pageIdx = ref(0)
const selectedBucket = ref("")
const bucketQuery = ref("")

const selectedFile = ref("")
const encrypted = ref(false)
const showModal = ref(false)
const chooseToContinue = ref(false)

EventsOn('sdconnectAvailable', () => {
    skipLogin.value = true;
})

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
        CheckEncryption(filename, selectedBucket.value).then((checks: Array<boolean>) => {
            console.log(checks)
            console.log(typeof checks)
            selectedFile.value = filename
            encrypted.value = checks[0]

            let exportRow: CDataTableData = {
                'name': {'value': filename.split('/').reverse()[0] + (!checks[0] ? ".c4gh" : "")}, 
                'folder': {'value': selectedBucket.value}
            };
            exportData.value = [];
            exportData.value.push(exportRow);

            if (checks[1]) { // If exists
                showModal.value = true;
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
    ExportFile(selectedFile.value, selectedBucket.value, encrypted.value).then(() => {
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

        <c-flex v-show="pageIdx == 0" id="no-export-page">
            <h2>Export is not possible</h2>
            <p v-if="skipLogin">You need to have project manager rights to export files.</p>
            <p v-else>
                Please log in to SD Connect with your CSC credentials. 
                Note that only CSC project managers have export rights.
            </p>
            <LoginForm v-if="!skipLogin"></LoginForm>
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
        </c-flex>
    </c-container>
</template>

<style scoped>
c-autocomplete {
    width: 500px;
}

#no-export-page {
    width: 500px;
}

#drop-area {
    border: 3px solid var(--csc-primary);
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