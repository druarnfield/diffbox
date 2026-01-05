import { Routes, Route } from 'react-router-dom'
import Layout from './components/layout/Layout'
import WorkflowPage from './pages/WorkflowPage'
import ModelsPage from './pages/ModelsPage'
import SettingsPage from './pages/SettingsPage'

function App() {
  return (
    <Routes>
      <Route path="/" element={<Layout />}>
        <Route index element={<WorkflowPage />} />
        <Route path="workflow/:type" element={<WorkflowPage />} />
        <Route path="models" element={<ModelsPage />} />
        <Route path="settings" element={<SettingsPage />} />
      </Route>
    </Routes>
  )
}

export default App
