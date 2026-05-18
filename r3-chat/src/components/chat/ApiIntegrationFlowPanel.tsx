import React, { useMemo } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  FileText,
  CheckCircle2,
  Circle,
  ChevronRight,
  Sparkles,
  Package,
  Loader2,
} from 'lucide-react';

interface ApiIntegrationFlowPanelProps {
  metadata?: Record<string, unknown>;
  onSendMessage: (text: string) => void;
  isStreaming?: boolean;
}

const REQUIRED_FIELDS_COUNT = 8;

const statusConfig: Record<string, { label: string; color: string; bg: string; border: string; icon: React.ReactNode }> = {
  collecting: {
    label: 'Collecting Info',
    color: 'text-amber-300',
    bg: 'bg-amber-500/10',
    border: 'border-amber-500/20',
    icon: <Circle className="w-3.5 h-3.5" />,
  },
  ready_to_generate: {
    label: 'Ready to Generate',
    color: 'text-blue-300',
    bg: 'bg-blue-500/10',
    border: 'border-blue-500/20',
    icon: <Sparkles className="w-3.5 h-3.5" />,
  },
  generated: {
    label: 'Generated',
    color: 'text-emerald-300',
    bg: 'bg-emerald-500/10',
    border: 'border-emerald-500/20',
    icon: <CheckCircle2 className="w-3.5 h-3.5" />,
  },
};

function safeStringArray(value: unknown): string[] {
  if (Array.isArray(value)) return value.filter((v): v is string => typeof v === 'string');
  return [];
}

function safeRecord(value: unknown): Record<string, string> {
  if (value && typeof value === 'object' && !Array.isArray(value)) {
    const result: Record<string, string> = {};
    for (const [k, v] of Object.entries(value)) {
      if (typeof v === 'string') result[k] = v;
    }
    return result;
  }
  return {};
}

function safeOptions(value: unknown): Record<string, string[]> {
  if (value && typeof value === 'object' && !Array.isArray(value)) {
    const result: Record<string, string[]> = {};
    for (const [k, v] of Object.entries(value)) {
      if (Array.isArray(v)) result[k] = v.filter((item): item is string => typeof item === 'string');
    }
    return result;
  }
  return {};
}

function formatFieldName(key: string): string {
  return key
    .replace(/_/g, ' ')
    .replace(/\b\w/g, (c) => c.toUpperCase());
}

export const ApiIntegrationFlowPanel: React.FC<ApiIntegrationFlowPanelProps> = ({
  metadata,
  onSendMessage,
  isStreaming = false,
}) => {
  const flow = metadata?.flow;
  if (flow !== 'api_integration') return null;

  const status = typeof metadata?.status === 'string' ? metadata.status : 'collecting';
  const collected = safeRecord(metadata?.collected_fields);
  const missing = safeStringArray(metadata?.missing_fields);
  const options = safeOptions(metadata?.suggested_options);
  const deliverables = safeStringArray(metadata?.deliverables);

  const collectedCount = Object.keys(collected).length;
  const progressPercent = Math.min(100, Math.round((collectedCount / REQUIRED_FIELDS_COUNT) * 100));
  const currentMissing = missing[0];
  const currentOptions = currentMissing ? options[currentMissing] || [] : [];
  const config = statusConfig[status] || statusConfig.collecting;

  const handleChipClick = (value: string) => {
    if (isStreaming) return;
    onSendMessage(value);
  };

  const handleGenerateClick = () => {
    if (isStreaming) return;
    onSendMessage('Generate the API integration project package now.');
  };

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="flex-shrink-0 px-4 py-3 border-b border-[var(--border-subtle)]">
        <div className="flex items-center gap-2">
          <Package className="w-4 h-4 text-[var(--accent-primary)]" />
          <span className="text-sm font-semibold text-[var(--text-primary)]">API Integration</span>
        </div>
        <div className={`mt-2 inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium border ${config.bg} ${config.color} ${config.border}`}>
          {config.icon}
          {config.label}
        </div>
      </div>

      {/* Scrollable content */}
      <div className="flex-1 overflow-y-auto px-4 py-4 space-y-5">
        {/* Progress */}
        <div>
          <div className="flex items-center justify-between text-xs mb-1.5">
            <span className="text-[var(--text-secondary)]">Progress</span>
            <span className="text-[var(--text-primary)] font-medium">{collectedCount}/{REQUIRED_FIELDS_COUNT}</span>
          </div>
          <div className="h-2 rounded-full bg-[var(--bg-elevated)] overflow-hidden">
            <motion.div
              className="h-full rounded-full bg-[var(--accent-primary)]"
              initial={{ width: 0 }}
              animate={{ width: `${progressPercent}%` }}
              transition={{ duration: 0.5, ease: 'easeOut' }}
            />
          </div>
        </div>

        {/* Current missing field */}
        <AnimatePresence mode="wait">
          {currentMissing && status === 'collecting' && (
            <motion.div
              key={currentMissing}
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -8 }}
              transition={{ duration: 0.25 }}
            >
              <p className="text-xs text-[var(--text-secondary)] mb-2">Current question</p>
              <div className="flex items-center gap-1.5 text-sm text-[var(--text-primary)] font-medium">
                <ChevronRight className="w-3.5 h-3.5 text-[var(--accent-primary)]" />
                {formatFieldName(currentMissing)}
              </div>
            </motion.div>
          )}
        </AnimatePresence>

        {/* Suggested options chips */}
        <AnimatePresence>
          {currentOptions.length > 0 && status === 'collecting' && (
            <motion.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 'auto' }}
              exit={{ opacity: 0, height: 0 }}
            >
              <p className="text-xs text-[var(--text-secondary)] mb-2">Suggested options</p>
              <div className="flex flex-wrap gap-1.5">
                {currentOptions.map((option) => (
                  <button
                    key={option}
                    onClick={() => handleChipClick(option)}
                    disabled={isStreaming}
                    className="px-2.5 py-1 text-xs rounded-full bg-white/[0.04] text-[var(--text-secondary)] border border-[var(--border-subtle)] hover:bg-white/[0.08] hover:text-[var(--text-primary)] hover:border-[var(--border-hover)] transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    {option}
                  </button>
                ))}
              </div>
            </motion.div>
          )}
        </AnimatePresence>

        {/* Collected fields summary */}
        {collectedCount > 0 && (
          <div>
            <p className="text-xs text-[var(--text-secondary)] mb-2">Collected fields</p>
            <div className="flex flex-wrap gap-1.5">
              {Object.entries(collected).map(([key, value]) => (
                <div
                  key={key}
                  className="inline-flex items-center gap-1 px-2 py-1 text-xs rounded-md bg-emerald-500/10 text-emerald-200 border border-emerald-500/15"
                  title={value}
                >
                  <CheckCircle2 className="w-3 h-3" />
                  <span className="truncate max-w-[120px]">{formatFieldName(key)}</span>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Ready to generate CTA */}
        <AnimatePresence>
          {status === 'ready_to_generate' && (
            <motion.div
              initial={{ opacity: 0, scale: 0.96 }}
              animate={{ opacity: 1, scale: 1 }}
              exit={{ opacity: 0, scale: 0.96 }}
              transition={{ duration: 0.3 }}
            >
              <button
                onClick={handleGenerateClick}
                disabled={isStreaming}
                className="w-full flex items-center justify-center gap-2 px-4 py-2.5 rounded-xl bg-[var(--accent-primary)] text-white text-sm font-medium hover:opacity-90 transition-opacity disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {isStreaming ? (
                  <>
                    <Loader2 className="w-4 h-4 animate-spin" />
                    Generating...
                  </>
                ) : (
                  <>
                    <Sparkles className="w-4 h-4" />
                    Generate project package
                  </>
                )}
              </button>
            </motion.div>
          )}
        </AnimatePresence>

        {/* Generated deliverables */}
        <AnimatePresence>
          {status === 'generated' && deliverables.length > 0 && (
            <motion.div
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -8 }}
            >
              <p className="text-xs text-[var(--text-secondary)] mb-2">Deliverables</p>
              <div className="space-y-1.5">
                {deliverables.map((filename) => (
                  <div
                    key={filename}
                    className="flex items-center gap-2 px-3 py-2 rounded-lg bg-white/[0.03] border border-[var(--border-subtle)]"
                  >
                    <FileText className="w-4 h-4 text-[var(--accent-primary)] flex-shrink-0" />
                    <span className="text-sm text-[var(--text-primary)] truncate">{filename}</span>
                  </div>
                ))}
              </div>
              {/* TODO: Link to artifact download when artifact system is wired for api_integration deliverables */}
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </div>
  );
};

export default ApiIntegrationFlowPanel;
