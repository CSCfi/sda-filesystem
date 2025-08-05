<script lang="ts" setup>
import { ref, watch, computed } from "vue";
import { airlock } from "../../wailsjs/go/models";
import { EventsOn, EventsEmit, OnFileDrop, OnFileDropOff } from "../../wailsjs/runtime/runtime";
import { CAutocompleteItem, CDataTableHeader, CPaginationOptions } from "@cscfi/csc-ui/dist/types";
import { SelectFiles, CheckObjectExistences, CheckBucketExistence, ExportFiles, WalkDirs } from "../../wailsjs/go/main/App";
import { mdiTrashCanOutline } from "@mdi/js";
import { ValidationHelperType, ValidationResult } from "../types/common";
import ValidationHelper from "../components/ValidationHelper.vue";

const exportHeaders: CDataTableHeader[] = [
  { key: "name", value: "Name", sortable: false },
  { key: "bucket", value: "Destination bucket", sortable: false },
];

const exportHeadersModifiable: CDataTableHeader[] = [
  { key: "name", value: "Name" },
  { key: "bucket", value: "Destination bucket", sortable: false },
  { key: "actions", value: null, sortable: false, justify: "end"},
];


const bucketItems = ref<CAutocompleteItem[]>([]);
const filteredBucketItems = ref<CAutocompleteItem[]>([]);

const pageIdx = ref(0);
const selectedBucket = ref("");
const bucketQuery = ref("");
const selectedFolder = ref("");
const uniqueBucket = ref<boolean | undefined>(undefined); // true if user is allowed to create the bucket, false if they already own it
// eslint-disable-next-line no-undef
const exportAutocomplete = ref<HTMLCAutocompleteElement | null>(null);

const validationHelperData = ref<ValidationHelperType[]>([
  { check: "lowerCaseOrNum", message: "Bucket name should start with a lowercase letter or a number.", type: "info"},
  { check: "inputLength", message: "Bucket name should be between 3 and 63 characters long.", type: "info"},
  { check: "alphaNumHyphen", message: "Use Latin letters (a-z), numbers (0-9) and a hyphen (-).", type: "info"},
  { check: "alphaNumHyphen", message: "Uppercase letters, underscore (_) and accent letters with diacritics or special marks (áäöé) are not allowed.", type: "info"},
  { check: "ownable", message: "Bucket names must be unique across all existing folders in all projects in SD Connect and Allas.", type: "info"}
]);

const selectedSet = ref<airlock.UploadSet>({bucket: "", files: [], objects: [], exists: []});
const selectedPrefix = computed(() => selectedBucket.value + (selectedFolder.value ? "/" + selectedFolder.value : ""));
const filesToOverwrite = computed(() => selectedSet.value.exists.includes(true));
const isDraggingFile = ref<boolean>(false);
let debounceTimer: ReturnType<typeof setTimeout> | null = null;

const paginationOptions: CPaginationOptions = {
  itemCount: selectedSet.value.files.length,
  itemsPerPage: 5,
  currentPage: 1,
  startFrom: 0,
  endTo: 4,
};

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
  return selectedSet.value.objects.map((object: string) => {
    return {
      name: {value: object },
      bucket: {value: selectedSet.value.bucket},
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
              {
                let idx = selectedSet.value.objects.indexOf(object);
                selectedSet.value.objects.splice(idx, 1);
                selectedSet.value.files.splice(idx, 1);
                selectedSet.value.exists.splice(idx, 1);
              }
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

async function addFiles(paths: string[]) {
  if (!paths || !paths.length) {
    return;
  }

  try {
    let set: airlock.UploadSet =
      await WalkDirs(paths, selectedSet.value.objects, selectedPrefix.value);
    let exists: boolean[] = set.exists;
    if (!uniqueBucket.value) {
      exists = await CheckObjectExistences(set);
    }

    selectedSet.value.bucket = set.bucket;
    selectedSet.value.objects.push(...set.objects);
    selectedSet.value.files.push(...set.files);
    selectedSet.value.exists.push(...exists);
  } catch (e) {
    EventsEmit("showToast", "File selection failed", e as string);
  }
}

function selectFiles() {
  // TODO select files and folders
  SelectFiles().then((files: string[]) => {
    addFiles(files);
  });
}

function containsFilterString(str: string): boolean {
  return str.toLowerCase().includes(selectedBucket.value.toLowerCase());
}

async function validateBucketInput(input: string): Promise<ValidationResult> {
  if (!input) {
    return {
      lowerCaseOrNum: undefined,
      inputLength: undefined,
      alphaNumHyphen: undefined,
      ownable: undefined,
    };
  }

  let result: ValidationResult = {
    lowerCaseOrNum: !!(input[0].match(/[\p{L}0-9]/u) && input[0] === input[0].toLowerCase()),
    inputLength: input.length >= 3 && input.length <= 63,
    alphaNumHyphen: !!input.match(/^[a-z0-9-]+$/g),
    ownable: undefined,
  };

  if (result.lowerCaseOrNum && result.inputLength && result.alphaNumHyphen) {
    try {
      uniqueBucket.value = !(await CheckBucketExistence(selectedBucket.value));
      result.ownable = true;
    } catch (e) {
      result.ownable = false;
      EventsEmit("showToast", "Bucket cannot be selected", e as string);
    }
  }

  return result;
}

function exportFiles() {
  window.scrollTo({top: 0});
  ExportFiles(selectedSet.value, !uniqueBucket.value).then(() => {
    pageIdx.value = 4;
  }).catch((_e) => {
    pageIdx.value = 2;
    EventsEmit("showToast", "Export interrupted", "Check logs for further details");
  });
}

function validateFolderInput(input: string): boolean {
  return !input || !!input.match(/^[^/]+(\/[^/]+)*$/);
}

function deleteExisting() {
  selectedSet.value.files = selectedSet.value.files.filter(
    (_, i) => !selectedSet.value.exists[i]
  );
  selectedSet.value.objects = selectedSet.value.objects.filter(
    (_, i) => !selectedSet.value.exists[i]
  );
  selectedSet.value.exists = selectedSet.value.exists.filter(
    (_, i) => !selectedSet.value.exists[i]
  );
}

function clearSet() {
  selectedSet.value.bucket = "";
  selectedSet.value.files = [];
  selectedSet.value.objects = [];
  selectedSet.value.exists = [];
}

function reset() {
  exportAutocomplete.value?.reset();
  uniqueBucket.value = undefined;
  bucketQuery.value = "";
  selectedBucket.value = "";
  selectedFolder.value = "";
  clearSet();
  pageIdx.value = 1;
}

</script>

<template>
  <div class="container">
    <c-steps :value="pageIdx" :style="{display: pageIdx ? 'block' : 'none'}">
      <c-step>Choose bucket</c-step>
      <c-step>Export files</c-step>
      <c-step>Export complete</c-step>
    </c-steps>

    <c-modal :value="filesToOverwrite" disable-backdrop-blur>
      <c-card>
        <c-card-title>Objects already exist</c-card-title>

        <c-card-content>
          You have selected files that would overwrite objects
          that already exists in SD Connect. Overwrite objects?
        </c-card-content>

        <c-card-actions justify="end">
          <c-button outlined @click="deleteExisting">
            Cancel and discard files
          </c-button>
          <c-button @click="selectedSet.exists.fill(false)">
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
        ref="exportAutocomplete"
        v-model="selectedBucket"
        v-control
        label="Bucket name"
        :items="filteredBucketItems"
        items-per-page="5"
        spellcheck="false"
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
              spellcheck="false"
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
          <c-button id="select-files-button" outlined @click="selectFiles()">
            Select files
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
        :pagination="paginationOptions"
        :hide-footer="selectedSet.files.length <= 5"
      />
      <c-row justify="space-between">
        <c-button outlined @click="pageIdx--; clearSet()">
          Cancel
        </c-button>
        <c-button
          :disabled="!selectedSet.files.length"
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
        @click="reset"
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
  padding: 38px;
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
