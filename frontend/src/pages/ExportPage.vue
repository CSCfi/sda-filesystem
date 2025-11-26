<script lang="ts" setup>
import { ref, watch, computed } from "vue";
import { airlock } from "../../wailsjs/go/models";
import { EventsOn, EventsEmit, OnFileDrop, OnFileDropOff } from "../../wailsjs/runtime/runtime";
import { CAutocompleteItem, CDataTableHeader, CPaginationOptions } from "@cscfi/csc-ui/dist/types";
import {
  SelectFiles,
  CheckObjectExistences,
  CheckBucketExistence,
  ExportFiles,
  WalkDirs,
  ValidateEmail,
} from "../../wailsjs/go/main/App";
import { mdiTrashCanOutline } from "@mdi/js";
import { ValidationHelperType, ValidationResult } from "../types/common";
import ValidationHelper from "../components/ValidationHelper.vue";

const exportHeaders: CDataTableHeader[] = [
  { key: "name", value: "Name", sortable: false },
  { key: "path", value: "Path", sortable: false },
];

const exportHeadersModifiable: CDataTableHeader[] = [
  { key: "name", value: "Name" },
  { key: "path", value: "Path" },
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
// eslint-disable-next-line no-undef
const exportTable = ref<HTMLCDataTableElement | null>(null);

const validationHelperData = ref<ValidationHelperType[]>([
  { check: "lowerCaseOrNum", message: "Bucket name should start and end with a lowercase letter or a number.", type: "info"},
  { check: "inputLength", message: "Bucket name should be between 3 and 63 characters long.", type: "info"},
  { check: "alphaNumHyphen", message: "Use only Latin letters (a-z), numbers (0-9), and hyphens (-).", type: "info"},
  { check: "alphaNumHyphen", message: "Uppercase letters, underscore (_) and accent letters with diacritics or special marks (áäöé) are not allowed.", type: "info"},
  { check: "ownable", message: "Bucket names must be unique across all existing folders in all projects in SD Connect and Allas.", type: "info"}
]);

const selectedSet = ref<airlock.UploadSet>({bucket: "", files: [], objects: [], exists: []});
const selectedPrefix = computed(() => selectedBucket.value + (selectedFolder.value ? "/" + selectedFolder.value : ""));
const filesToOverwrite = computed(() => selectedSet.value.exists.includes(true));
const isDraggingFile = ref<boolean>(false);
let debounceTimer: ReturnType<typeof setTimeout> | null = null;

const isFindata = ref<boolean>(false);
const validEmail = ref<boolean>(true);
const defaultEmail = ref<string>("");
const selectedEmail = ref<string>("");
const parsedEmail = ref<string>("");
const selectedJournalNumber = ref<string>("");

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

EventsOn("findataProject", (email: string) => {
  isFindata.value = true;
  defaultEmail.value = email;
  selectedEmail.value = email;
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
    const withoutC4gh = object.endsWith(".c4gh") ? object.slice(0, -5) : object;
    return {
      name: { value: withoutC4gh.split("/").pop() },
      path: { value: selectedBucket.value + "/" + withoutC4gh },
      actions: {
        children: [
        {
          value: "Remove from list",
          component: {
          tag: "c-button",
          params: {
            text: true,
            size: "small",
            title: "Remove from list",
            onClick: () =>
              {
                let idx = selectedSet.value.objects.indexOf(object);
                selectedSet.value.objects.splice(idx, 1);
                selectedSet.value.files.splice(idx, 1);
                selectedSet.value.exists.splice(idx, 1);

                const startFrom = exportTable.value?.pagination.startFrom;
                const endTo = exportTable.value?.pagination.endTo;
                const count = exportTable.value?.pagination.itemCount;
                const currentPage = exportTable.value?.pagination.currentPage;
                const perPage = exportTable.value?.pagination.itemsPerPage;

                if (startFrom && endTo && count && currentPage && perPage &&
                    exportTable.value && count - 2 < startFrom) {
                  exportTable.value.pagination.currentPage = currentPage - 1;
                  exportTable.value.pagination.startFrom = startFrom - perPage;
                  exportTable.value.pagination.endTo = endTo - perPage;
                }
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
              value: "Remove from list",
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

watch(() => selectedEmail.value, () => {
  validEmail.value = true;
})

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

  function isLowerCaseOrNum(char: string) {
    return /[\p{L}0-9]/u.test(char) && char === char.toLowerCase();
  }

  let result: ValidationResult = {
    lowerCaseOrNum: isLowerCaseOrNum(input[0]) &&
      isLowerCaseOrNum(input[input.length - 1]),
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

function toFileSelectionPage() {
  if (!isFindata.value) {
    pageIdx.value++;

    return;
  }

  ValidateEmail(selectedEmail.value).then((email: string) => {
    parsedEmail.value = email;

    if (email != "") {
      pageIdx.value++;
    } else {
      validEmail.value = false;
      window.scrollTo({top: 0});
    }
  });
}

function exportFiles() {
  window.scrollTo({top: 0});
  let metadata: { [key:string]:string } = {};
  if (isFindata.value) {
    metadata = {
      "journal_number": selectedJournalNumber.value,
      "author_email":   parsedEmail.value,
    }
  }
  ExportFiles(selectedSet.value, !uniqueBucket.value, metadata).then(() => {
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
  selectedEmail.value = defaultEmail.value;
  selectedJournalNumber.value = "";
  validEmail.value = true;
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
      <div v-show="isFindata">
        <h3>Details for Findata</h3>
        <p>
          In secondary use projects one copy of the exported data will be uploaded to SD Connect
          and one copy will be automatically transferred to Findata for scrutiny.
        </p>
        <c-text-field
          v-model="selectedJournalNumber"
          v-control
          label="Journal number"
          spellcheck="false"
          trim-whitespace
          required
        />
        <c-text-field
          v-model="selectedEmail"
          v-control
          label="Your email"
          :valid="validEmail"
          validation="Email is invalid"
          spellcheck="false"
          trim-whitespace
          required
        />
      </div>
      <h3>Select or create a destination bucket</h3>
      <p>
        Bucket, folder and file names cannot be changed after creation or upload.
        Remember, all bucket names are public; please do not include any confidential information.
      </p>
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
            validateFolderInput(selectedFolder) === false ||
            (isFindata && (selectedEmail == '' || selectedJournalNumber == ''))
        "
        @click="toFileSelectionPage"
      >
        Continue
      </c-button>
    </div>
    <div v-show="pageIdx == 2">
      <p><strong>Destination bucket: </strong>{{ selectedBucket }}</p>
      <p><strong>Destination folder: </strong>{{ selectedFolder ? selectedFolder : '-' }}</p>
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
        <p>
          In secondary use projects one copy of the exported data will be uploaded to
          SD Connect and one copy will be automatically transferred to Findata for scrutiny.
        </p>
      </div>
      <c-data-table
        v-if="exportData.length"
        id="export-table"
        ref="exportTable"
        class="gateway-table"
        :data.prop="exportData"
        :headers.prop="exportHeadersModifiable"
        :pagination="paginationOptions"
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
        :pagination="paginationOptions"
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
