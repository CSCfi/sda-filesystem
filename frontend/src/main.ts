import { createApp } from 'vue';
import { defineCustomElements } from '@cscfi/csc-ui/loader';
import { vControl } from '@cscfi/csc-ui-vue';
import '@mdi/font/css/materialdesignicons.css';
import '@cscfi/csc-ui/css/theme.css';

import App from './App.vue';
import Access from './pages/AccessPage.vue'
import Export from './pages/ExportPage.vue'
import Select from './pages/SelectPage.vue'
import Logs from './pages/LogsPage.vue'

const app = createApp(App);
app.component('Access', Access);
app.component('Export', Export);
app.component('Select', Select);
app.component('Logs', Logs);

app.directive('control', vControl);
defineCustomElements();

app.mount('#app');
