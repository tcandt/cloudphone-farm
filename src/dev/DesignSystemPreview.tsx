import React, { useState } from 'react';
import { BrandLogo } from '@brand/BrandLogo';
import { Button, Card, Badge, Modal, ConfirmDialog, ToastProvider, useToastStore, Loading, EmptyState, ErrorState, ErrorBoundary } from '@ui/index';
import { brandTokens } from '@brand/tokens';
import { Settings, Shield, Zap } from 'lucide-react';

export const DesignSystemPreview: React.FC = () => {
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [isConfirmOpen, setIsConfirmOpen] = useState(false);
  const [isSimulatingError, setIsSimulatingError] = useState(false);
  const addToast = useToastStore((state) => state.addToast);

  const triggerError = () => {
    setIsSimulatingError(true);
    setTimeout(() => setIsSimulatingError(false), 2000);
  };

  return (
    <div className={`min-h-screen ${brandTokens.colors.appBg} p-8`}>
      <ToastProvider />
      <div className="max-w-7xl mx-auto space-y-12">
        {/* Header */}
        <div className="flex items-center gap-4 border-b border-slate-100 pb-6">
          <BrandLogo size="lg" />
          <h1 className="text-2xl font-bold text-slate-800 border-l border-slate-200 pl-4">Design System Preview</h1>
        </div>

        {/* Brand Tokens */}
        <section>
          <h2 className="text-lg font-bold text-slate-800 mb-4">Brand Tokens</h2>
          <div className="flex gap-4">
            <div className="w-24 h-24 rounded-2xl bg-emerald-600 shadow-sm flex items-center justify-center text-white text-xs font-bold">Primary</div>
            <div className="w-24 h-24 rounded-2xl bg-emerald-50 text-emerald-700 flex items-center justify-center text-xs font-bold border border-emerald-100">Light</div>
            <div className="w-24 h-24 rounded-2xl bg-slate-50 text-slate-500 flex items-center justify-center text-xs font-bold border border-slate-200">Workspace</div>
          </div>
        </section>

        {/* Buttons */}
        <section>
          <h2 className="text-lg font-bold text-slate-800 mb-4">Buttons</h2>
          <div className="flex flex-wrap gap-4 items-center">
            <Button variant="primary">Primary</Button>
            <Button variant="secondary">Secondary</Button>
            <Button variant="outline">Outline</Button>
            <Button variant="ghost">Ghost</Button>
            <Button variant="danger">Danger</Button>
            <Button variant="primary" isLoading>Loading</Button>
            <Button variant="primary" size="sm">Small</Button>
            <Button variant="primary" size="lg">Large Button</Button>
          </div>
        </section>

        {/* Badges */}
        <section>
          <h2 className="text-lg font-bold text-slate-800 mb-4">Badges</h2>
          <div className="flex flex-wrap gap-4 items-center">
            <Badge variant="primary">Primary</Badge>
            <Badge variant="success">Online</Badge>
            <Badge variant="warning">Warning</Badge>
            <Badge variant="danger">Error</Badge>
            <Badge variant="neutral">Draft</Badge>
            <Badge variant="success" size="sm">Tiny</Badge>
          </div>
        </section>

        {/* Cards */}
        <section>
          <h2 className="text-lg font-bold text-slate-800 mb-4">Cards & Shadows</h2>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            <Card>
              <h3 className="font-bold text-slate-800 mb-2">Default Card</h3>
              <p className="text-slate-500 text-sm">Cards use a 20px border radius and very soft shadows.</p>
            </Card>
            <Card className="bg-emerald-50 border-emerald-100">
              <h3 className="font-bold text-emerald-800 mb-2">Tinted Card</h3>
              <p className="text-emerald-600 text-sm">Useful for highlighted information or active states.</p>
            </Card>
            <Card padding="lg">
              <h3 className="font-bold text-slate-800 mb-2">Large Padding</h3>
              <p className="text-slate-500 text-sm">Using padding="lg" for more spacious content areas.</p>
            </Card>
          </div>
        </section>

        {/* Interactions (Modal & Toast) */}
        <section>
          <h2 className="text-lg font-bold text-slate-800 mb-4">Interactions</h2>
          <div className="flex gap-4">
            <Button onClick={() => setIsModalOpen(true)}>Open Modal</Button>
            <Button variant="danger" onClick={() => setIsConfirmOpen(true)}>Open Confirm</Button>
            <Button onClick={() => addToast({ type: 'success', title: 'Action Successful', message: 'The item was updated correctly.' })}>Success Toast</Button>
            <Button onClick={() => addToast({ type: 'error', title: 'Action Failed' })} variant="danger">Error Toast</Button>
          </div>

          <Modal
            isOpen={isModalOpen}
            onClose={() => setIsModalOpen(false)}
            title="Design System Modal"
            footer={
              <>
                <Button variant="ghost" onClick={() => setIsModalOpen(false)}>Cancel</Button>
                <Button onClick={() => { addToast({ type: 'success', title: 'Saved' }); setIsModalOpen(false); }}>Save Changes</Button>
              </>
            }
          >
            <p className="text-slate-600 mb-4">This modal uses the standard border radius and soft backdrop blur.</p>
            <Card className="bg-slate-50 border-none shadow-none p-4">
              <p className="text-sm font-medium text-slate-700">Inner content goes here.</p>
            </Card>
          </Modal>

          <ConfirmDialog
            isOpen={isConfirmOpen}
            onClose={() => setIsConfirmOpen(false)}
            title="Delete Device?"
            description="Are you sure you want to delete this device? This action cannot be undone."
            variant="danger"
            confirmLabel="Delete"
            onConfirm={() => {
              addToast({ type: 'info', title: 'Device deleted' });
              setIsConfirmOpen(false);
            }}
          />
        </section>

        {/* States */}
        <section>
          <h2 className="text-lg font-bold text-slate-800 mb-4">States (Loading, Empty, Error)</h2>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            <Card className="flex items-center justify-center py-12">
              <Loading text="Loading devices..." />
            </Card>
            <Card className="p-0 overflow-hidden">
              <EmptyState 
                title="No Devices Found" 
                description="You haven't enrolled any physical devices to this group yet."
                icon={<Settings size={32} />}
                action={<Button size="sm">Enroll Device</Button>}
              />
            </Card>
            <Card className="p-0 overflow-hidden">
              <ErrorBoundary>
                {isSimulatingError ? (
                  <ThrowError />
                ) : (
                  <div className="flex flex-col items-center justify-center p-12 text-center h-full">
                    <p className="text-slate-600 mb-4">Component works fine.</p>
                    <Button variant="outline" size="sm" onClick={triggerError}>Simulate Crash</Button>
                  </div>
                )}
              </ErrorBoundary>
            </Card>
          </div>
        </section>
      </div>
    </div>
  );
};

const ThrowError = () => {
  throw new Error("Simulated rendering error.");
};
