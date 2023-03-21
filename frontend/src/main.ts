import { createApp } from 'vue';
import { applyPolyfills, defineCustomElements } from 'csc-ui/loader';
import { vControl } from 'csc-ui-vue-directive';

import App from './App.vue';
import Access from './pages/AccessPage.vue'
import Export from './pages/ExportPage.vue'
import Login from './pages/LoginPage.vue'
import Logs from './pages/LogsPage.vue'

const app = createApp(App);
app.component('Access', Access);
app.component('Export', Export);
app.component('Login', Login);
app.component('Logs', Logs);

app.directive('control', vControl);

applyPolyfills().then(() => {
  defineCustomElements();
});

app.mount('#app');