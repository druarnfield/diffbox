import { useState } from 'react'

export default function SettingsPage() {
  const [hfToken, setHfToken] = useState('')
  const [civitaiToken, setCivitaiToken] = useState('')

  return (
    <div className="max-w-2xl space-y-8">
      <h1 className="text-2xl font-bold">Settings</h1>

      {/* API Tokens */}
      <section className="space-y-4">
        <h2 className="text-lg font-semibold">API Tokens</h2>
        <p className="text-sm text-muted-foreground">
          Configure your API tokens for downloading models from HuggingFace and
          Civitai.
        </p>

        <div className="space-y-4">
          <div className="space-y-2">
            <label className="text-sm font-medium">HuggingFace Token</label>
            <input
              type="password"
              value={hfToken}
              onChange={(e) => setHfToken(e.target.value)}
              placeholder="hf_xxxxx"
              className="w-full px-4 py-2 border rounded-md"
            />
            <p className="text-xs text-muted-foreground">
              Get your token from{' '}
              <a
                href="https://huggingface.co/settings/tokens"
                target="_blank"
                rel="noopener noreferrer"
                className="text-primary hover:underline"
              >
                huggingface.co/settings/tokens
              </a>
            </p>
          </div>

          <div className="space-y-2">
            <label className="text-sm font-medium">Civitai API Key</label>
            <input
              type="password"
              value={civitaiToken}
              onChange={(e) => setCivitaiToken(e.target.value)}
              placeholder="xxxxx"
              className="w-full px-4 py-2 border rounded-md"
            />
            <p className="text-xs text-muted-foreground">
              Get your key from{' '}
              <a
                href="https://civitai.com/user/account"
                target="_blank"
                rel="noopener noreferrer"
                className="text-primary hover:underline"
              >
                civitai.com/user/account
              </a>
            </p>
          </div>

          <button className="px-4 py-2 bg-primary text-primary-foreground rounded-md hover:bg-primary/90 transition-colors">
            Save Tokens
          </button>
        </div>
      </section>

      {/* Config Export/Import */}
      <section className="space-y-4">
        <h2 className="text-lg font-semibold">Configuration</h2>
        <p className="text-sm text-muted-foreground">
          Export your settings and pinned models for quick setup on new
          instances.
        </p>

        <div className="flex gap-4">
          <button className="px-4 py-2 border rounded-md hover:bg-muted transition-colors">
            Export Config
          </button>
          <label className="px-4 py-2 border rounded-md hover:bg-muted transition-colors cursor-pointer">
            Import Config
            <input type="file" accept=".json" className="hidden" />
          </label>
        </div>
      </section>

      {/* Default Settings */}
      <section className="space-y-4">
        <h2 className="text-lg font-semibold">Default Settings</h2>
        <p className="text-sm text-muted-foreground">
          Set default values for workflow parameters.
        </p>

        <div className="space-y-6">
          {/* I2V Defaults */}
          <div className="p-4 border rounded-lg space-y-4">
            <h3 className="font-medium">Wan 2.2 I2V</h3>
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <label className="text-sm">Default Steps</label>
                <input
                  type="number"
                  defaultValue={50}
                  className="w-full px-3 py-2 border rounded-md"
                />
              </div>
              <div className="space-y-2">
                <label className="text-sm">Default CFG</label>
                <input
                  type="number"
                  defaultValue={5.0}
                  step={0.1}
                  className="w-full px-3 py-2 border rounded-md"
                />
              </div>
            </div>
          </div>

          {/* SVI Defaults */}
          <div className="p-4 border rounded-lg space-y-4">
            <h3 className="font-medium">SVI 2.0 Pro</h3>
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <label className="text-sm">Motion Frames</label>
                <input
                  type="number"
                  defaultValue={5}
                  className="w-full px-3 py-2 border rounded-md"
                />
              </div>
              <div className="space-y-2">
                <label className="text-sm">Default Clips</label>
                <input
                  type="number"
                  defaultValue={10}
                  className="w-full px-3 py-2 border rounded-md"
                />
              </div>
            </div>
          </div>

          {/* Qwen Defaults */}
          <div className="p-4 border rounded-lg space-y-4">
            <h3 className="font-medium">Qwen Image Edit</h3>
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <label className="text-sm">Default Steps</label>
                <input
                  type="number"
                  defaultValue={30}
                  className="w-full px-3 py-2 border rounded-md"
                />
              </div>
              <div className="space-y-2">
                <label className="text-sm">Default CFG</label>
                <input
                  type="number"
                  defaultValue={4.0}
                  step={0.1}
                  className="w-full px-3 py-2 border rounded-md"
                />
              </div>
            </div>
          </div>
        </div>

        <button className="px-4 py-2 bg-primary text-primary-foreground rounded-md hover:bg-primary/90 transition-colors">
          Save Defaults
        </button>
      </section>
    </div>
  )
}
