import { createApp } from 'vue';
import { defineCustomElements } from '@cscfi/csc-ui/loader';
import { vControl } from '@cscfi/csc-ui-vue';
import '@mdi/font/css/materialdesignicons.css';
import '@cscfi/csc-ui/css/theme.css';

import App from './App.vue';
import AccessPage from './pages/AccessPage.vue'
import ExportPage from './pages/ExportPage.vue'
import SelectPage from './pages/SelectPage.vue'
import LogsPage from './pages/LogsPage.vue'

const app = createApp(App);
app.component('AccessPage', AccessPage);
app.component('ExportPage', ExportPage);
app.component('SelectPage', SelectPage);
app.component('LogsPage', LogsPage);

app.directive('control', vControl);
defineCustomElements();

app.mount('#app');
