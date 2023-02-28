import { createApp } from 'vue';
import { applyPolyfills, defineCustomElements } from 'csc-ui/loader';
import { vControl } from 'csc-ui-vue-directive';

import App from './App.vue';
import Access from './components/Access.vue'
import Export from './components/Export.vue'
import Login from './components/Login.vue'
import Logs from './components/Logs.vue'

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