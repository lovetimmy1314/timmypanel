// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

import { createApp } from 'vue'
import { createPinia } from 'pinia'
import {
  create,
  NAlert,
  NButton,
  NCheckbox,
  NColorPicker,
  NConfigProvider,
  NDialogProvider,
  NDivider,
  NDropdown,
  NForm,
  NFormItem,
  NInput,
  NInputGroup,
  NModal,
  NRadioButton,
  NRadioGroup,
  NSelect,
  NSlider,
  NSpin,
  NSwitch,
  NTabPane,
  NTabs,
  NTag,
  NTooltip,
  NUpload,
  NUploadDragger,
  NMessageProvider,
} from 'naive-ui'
import App from './App.vue'
import router from './router'
import './style.css'
// 图标集要在任何 <Icon> 渲染之前注册好，并且关掉 iconify 的在线 API（决策 019）。
import './icons'

// 只注册用到的组件，而不是 app.use(naive) 全量安装 —— 全量会把整个组件库
// 打进首屏 chunk，这里能省掉大约一半体积。新增组件时记得同步加到这份清单。
const naive = create({
  components: [
    NAlert, NButton, NCheckbox, NColorPicker, NConfigProvider, NDialogProvider,
    NDivider, NDropdown, NForm, NFormItem, NInput,
    NInputGroup, NMessageProvider, NModal, NRadioButton, NRadioGroup, NSelect,
    NSlider, NSpin, NSwitch, NTabPane, NTabs, NTag, NTooltip, NUpload, NUploadDragger,
  ],
})

createApp(App).use(createPinia()).use(router).use(naive).mount('#app')
