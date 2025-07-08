<script lang="ts" setup>
import { ref, watch, computed } from "vue";
import { EventsOn, EventsEmit, OnFileDrop, OnFileDropOff } from "../../wailsjs/runtime/runtime";
import { CAutocompleteItem, CDataTableHeader } from "@cscfi/csc-ui/dist/types";
import { SelectFile, CheckFileExistence, CheckBucketExistence, ExportFile } from "../../wailsjs/go/main/App";
import { mdiTrashCanOutline } from "@mdi/js";
import { ValidationHelperType, ValidationResult } from "../types/common";
import ValidationHelper from "../components/ValidationHelper.vue";

const exportHeaders: CDataTableHeader[] = [
  { key: "name", value: "Name", sortable: false },
  { key: "folder", value: "Destination folder", sortable: false },
];

const exportHeadersModifiable: CDataTableHeader[] = [
  { key: "name", value: "Name", sortable: false },
  { key: "folder", value: "Destination folder", sortable: false },
  { key: "actions", value: null, sortable: false, justify: "end"},
];

const bucketItems = ref<CAutocompleteItem[]>([]);
const filteredBucketItems = ref<CAutocompleteItem[]>([]);

const pageIdx = ref(0);
const selectedBucket = ref("");
const bucketQuery = ref("");

const validationHelperData = ref<ValidationHelperType[]>([
  { check: "lowerCaseOrNum", message: "Bucket name should start with a lowercase letter or a number.", type: "info"},
  { check: "inputLength", message: "Bucket name should be between 3 and 63 characters long.", type: "info"},
  { check: "alphaNumDash", message: "Use Latin letters (a-z), numbers (0-9) and a dash (-).", type: "info"},
  { check: "alphaNumDash", message: "Uppercase letters, underscore (_) and accent letters with diacritics or special marks (áäöé) are not allowed.", type: "info"},
  { check: "unique", message: "Bucket names must be unique across all existing folders in all projects in SD Connect and Allas.", type: "info"}
]);

const selectedFolder = ref("");
const selectedFiles = ref<string[]>([]);
const filesToOverwrite = ref<string[]>([]);
const isDraggingFile = ref<boolean>(false);
let debounceTimer: ReturnType<typeof setTimeout> | null = null;

EventsOn("exportPossible", () => {
  pageIdx.value = 1;
});

EventsOn("setBuckets", (buckets: string[]) => {
  bucketItems.value = buckets.map((bucket: string) => ({
    value: bucket,
    name: bucket,
  }));
  filteredBucketItems.value = bucketItems.value;
});

const exportData = computed(() => {
  return selectedFiles.value.map((filename) => {
    return {
      name: {value: filename.split("/").reverse()[0] + ".c4gh"},
      folder: {value: selectedBucket.value},
      actions: {
        children: [
        {
          value: "Remove",
          component: {
          tag: "c-button",
          params: {
            text: true,
            size: "small",
            title: "Remove",
            onClick: () =>
              { selectedFiles.value = selectedFiles.value.filter((file) => file !== filename); }
          }},
          children: [
            {
              value: "",
              component: {
                tag: "c-icon",
                params: {
                  path: mdiTrashCanOutline,
                },
              },
            },
            {
              value: "Remove",
              component: {
                tag: "span",
              },
            },
          ],
        }],
      },
    };
  });
});

watch(() => pageIdx.value, (newPage: number) => {
  if (newPage === 2) {
    OnFileDrop((_x, _y, paths) => {
      addFiles(paths);
    }, true);
  } else {
    // Disable file drop if the drop zone is hidden
    OnFileDropOff();
  }
});

watch(() => bucketQuery.value, (query: string) => {
  if (debounceTimer) {
    clearTimeout(debounceTimer);
  }

  debounceTimer = setTimeout(async () => {
    selectedBucket.value = query;
    filteredBucketItems.value = bucketItems.value.filter((item: CAutocompleteItem) => {
      if (selectedBucket.value) {
        return containsFilterString(item.name);
      }
      return true;
    });
    const result = await validateBucketInput(bucketQuery.value);
    validationHelperData.value.forEach((item) => {
      item.type = result[item.check as keyof ValidationResult] ?
        "success" : item.type = result[item.check as keyof ValidationResult]  === false ?
          "error" : "info";
    });
  }, 300);
});

async function addFiles(filenames: string[]) {
  for (const filename of filenames) {
    // Check if file name is unique to other added
    if (selectedFiles.value.find((file) => file === filename)) {
      EventsEmit("showToast", `Could not choose file ${filename}`, "File name is not unique");
      continue;
    }
    // Check if file name is unique to bucket
    try {
      const found = await CheckFileExistence(filename, selectedBucket.value);
      if (found) {
        filesToOverwrite.value.push(filename);
      } else {
        selectedFiles.value.push(filename);
      }
    } catch (e) {
      EventsEmit("showToast", "Could not choose file", e as string);
    }
  }
}

function selectFile() {
  // TODO selecting multiple files/folders
  SelectFile().then((filename: string) => {
    if (filename) addFiles([filename]);
  });
}

async function exportFiles() {
  // TODO exporting to subfolders; proper export for multiple files ?
  let success = true;
  for (const file of selectedFiles.value) {
    try {
      await ExportFile(file, selectedBucket.value);
    } catch (e)  {
      success = false;
      EventsEmit("showToast", `Exporting file ${file} failed`, e as string);
    };
  };
  pageIdx.value = success ? 4 : 2;
}

function containsFilterString(str: string): boolean {
  return str.toLowerCase().includes(selectedBucket.value.toLowerCase());
}

async function validateBucketInput(input: string): Promise<ValidationResult> {
  if (!input) {
    return {
      lowerCaseOrNum: undefined,
      inputLength: undefined,
      alphaNumDash: undefined,
      unique: undefined,
    };
  }
  let uniqueBucket: boolean;
  const existingBucket: boolean = !!bucketItems.value.find((item) => item.name === input);

  if (existingBucket) uniqueBucket = true;
  else uniqueBucket = !(await CheckBucketExistence(selectedBucket.value));

  return {
    lowerCaseOrNum: !!(input[0].match(/[\p{L}0-9]/u) && input[0] === input[0].toLowerCase()),
    inputLength: input.length >= 3 && input.length <= 63,
    alphaNumDash: !!input.match(/^[a-z0-9-]+$/g),
    unique: uniqueBucket,
  };
}

function validateFolderInput(input: string): boolean {
  return !input || !!input.match(/^[^/]+(\/[^/]+)*$/);
}

</script>

<template>
  <div class="container">
    <c-steps :value="pageIdx" :style="{display: pageIdx ? 'block' : 'none'}">
      <c-step>Choose bucket</c-step>
      <c-step>Export files</c-step>
      <c-step>Export complete</c-step>
    </c-steps>

    <c-modal :value="!!filesToOverwrite.length" disable-backdrop-blur>
      <c-card>
        <c-card-title>File already exists</c-card-title>

        <c-card-content>
          Airlock wants to upload file {{ filesToOverwrite[0] }}
          but a similar file already exists in SD Connect. Overwrite file?
        </c-card-content>

        <c-card-actions justify="end">
          <c-button outlined @click="filesToOverwrite.shift()">
            Cancel
          </c-button>
          <c-button @click="selectedFiles.push(filesToOverwrite[0]); filesToOverwrite.shift()">
            Overwrite and continue
          </c-button>
        </c-card-actions>
      </c-card>
    </c-modal>

    <div v-show="pageIdx == 0" id="no-export-page">
      <h2>Export is not possible</h2>
      <p>You need to have project manager rights to export files.</p>
    </div>
    <div v-show="pageIdx == 1">
      <h2>Export files to SD Connect</h2>
      <p>
        Bucket, folder and file names cannot be changed after creation or upload.
        Remember, all bucket names are public; please do not include any confidential information.
      </p>
      <h3>Select or create a destination bucket</h3>
      <p>
        Choose a bucket from the dropdown or create a new one by entering a name.
        It will be created at the root of your project.
      </p>
      <c-autocomplete
        v-model="selectedBucket"
        v-control
        label="Bucket name"
        :items="filteredBucketItems"
        items-per-page="5"
        no-matching-items-message="You are creating a new bucket"
        return-value
        @changeQuery="bucketQuery = $event.detail"
      />
      <ValidationHelper
        v-for="item in validationHelperData"
        :key="item.message"
        :type="item.type"
        :message="item.message"
      />
      <c-accordion value="">
        <c-accordion-item
          heading="Export into folder (optional)"
          value="foldername"
          class="accordion-item"
        >
          <p>
            To export file into a folder, type the path using "/" to separate levels
            (e.g. Folder1/Folder2).
            You can select an existing folder or create new ones inside the bucket.
          </p>
          <div>
            <c-text-field
              v-model="selectedFolder"
              v-control
              label="Folder names (optional)"
              :valid="validateFolderInput(selectedFolder)"
              validation="Folder name is invalid"
              trim-whitespace
            />
          </div>
        </c-accordion-item>
      </c-accordion>
      <c-button
        class="continue-button"
        :disabled="
          validationHelperData.some(item => item.type !== 'success') ||
            validateFolderInput(selectedFolder) === false
        "
        @click="pageIdx++"
      >
        Continue
      </c-button>
    </div>
    <div v-show="pageIdx == 2">
      <div
        id="drop-area"
        :class="{ 'dragging': isDraggingFile }"
        @dragover.prevent="isDraggingFile = true"
        @dragleave="isDraggingFile = false"
        @drop.prevent="isDraggingFile = false"
      >
        <c-row align="center" gap="20">
          <h4>Drag and drop files and folders here or</h4>
          <c-button id="select-files-button" outlined @click="selectFile()">
            Select files and folders
          </c-button>
        </c-row>
        <p>
          All exported files are encrypted by default but can be accessed
          and automatically decrypted by project members via SD Connect.
        </p>
      </div>
      <c-data-table
        v-if="exportData.length"
        id="export-table"
        class="gateway-table"
        :data.prop="exportData"
        :headers.prop="exportHeadersModifiable"
        hide-footer="true"
      />
      <c-row justify="space-between">
        <c-button outlined @click="pageIdx--; selectedFiles = [];">
          Cancel
        </c-button>
        <c-button
          :disabled="!selectedFiles.length"
          @click="pageIdx++; exportFiles()"
        >
          Export
        </c-button>
      </c-row>
    </div>
    <div v-show="pageIdx == 3">
      <h2>Exporting files to SD Connect</h2>
      <p>Please wait, this might take a few minutes.</p>
      <c-progress-bar indeterminate />
      <c-data-table
        class="gateway-table"
        :data.prop="exportData"
        :headers.prop="exportHeaders"
        hide-footer="true"
      />
    </div>
    <div v-show="pageIdx == 4">
      <h2>Export complete</h2>
      <p>
        All files have been uploaded to SD Connect. You can now
        close or minimise the window to continue working.
      </p>
      <c-button
        class="continue-button"
        @click="selectedFiles = []; pageIdx = 1"
      >
        New Export
      </c-button>
    </div>
  </div>
</template>

<style scoped>
c-autocomplete {
  margin-bottom: -0.5rem;
}

#no-export-page {
  width: 500px;
}

#drop-area {
  --wails-drop-target: drop;
  border: 1px dashed var(--c-primary-600);
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


#drop-area.dragging {
  border: 3px dashed var(--c-primary-600);
}

#export-table {
  margin-bottom: 20px;
}

#select-files-button {
  background-color: white;
}

.accordion-item {
  margin-top: 1rem;
}

.accordion-item > * {
  margin-top: 0;
  margin-left: -1rem;
}
</style>
