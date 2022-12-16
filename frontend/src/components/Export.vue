<script lang="ts" setup>
import { ref } from 'vue'
import { EventsOn } from '../../wailsjs/runtime'
import { CAutocompleteItem } from 'csc-ui/dist/types';

const pageIdx = ref(0)

const bucketItems = ref<CAutocompleteItem[]>([])

EventsOn('projectManager', () => {
    pageIdx.value = 1;
})

EventsOn('setBuckets', (buckets: string[]) => {
    bucketItems.value = buckets.map((bucket: string) => ({
        value: bucket,
        name: bucket,
    }))
})

function onDropped(e: DragEvent) {
    console.log('File(s) dropped');
    e.preventDefault();
    console.log(e.dataTransfer?.files)
}
</script>

<template>
    <c-container class="fill-width">
        <c-steps :value="pageIdx" :style="{display: pageIdx ? 'block' : 'none'}">
            <c-step>Choose directory</c-step>
            <c-step>Export files</c-step>
            <c-step>Export complete</c-step>
        </c-steps>

        <c-flex v-if="pageIdx == 0">
            <h2>Export is not possible</h2>
            <p>Your need to be project manager to export files.</p>
        </c-flex>
        <c-flex v-else-if="pageIdx == 1">
            <h2>Select a destination folder for your export</h2>
            <p>
                Your export will be sent to SD Connect. 
                If the folder does not already exist in SD Connect, it will be created.
                Please note that the folder name cannot be modified afterwards.
            </p>
            <c-row>
                <c-autocomplete
                    label="Folder name"
                    return-value>
                </c-autocomplete>
            </c-row>
            <c-button 
                class="continue-button" 
                size="large" 
                @click="pageIdx++">
                Continue
            </c-button>
        </c-flex>
        <c-flex v-else-if="pageIdx == 2">
            <div
                id="drop-area"
                class="fill-width"
                @drop="onDropped" 
                @dragenter.prevent 
                @dragover.prevent>
                <c-row align="center" gap="20px">
                    <h4>Drag or drop file here or</h4>
                    <c-button outlined>Select file</c-button>
                </c-row>
                <p>If you wish to export multiple files, please create a tar/zip file.</p>
            </div>
            <c-row justify="space-between">
                <c-button @click="pageIdx--" outlined>Cancel</c-button>
                <c-button @click="pageIdx++">Export</c-button>
            </c-row>
        </c-flex>
    </c-container>
</template>

<style>
c-autocomplete {
    width: 500px;
}

#drop-area {
    border: 3px dashed var(--csc-primary);
    height: 200px;
    margin-top: 20px;
    margin-bottom: 20px;
    display: flex;
    justify-content: center;
    align-items: center;
    flex-direction: column;
}
</style>