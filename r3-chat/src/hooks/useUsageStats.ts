import { useState, useEffect } from 'react';
import { API_BASE_URL } from '../constants';

interface UsageStats {
    todayMessages: number;
    todayTokens: number;
    totalMessages: number;
    totalTokens: number;
    tier: string;
    limits: {
        messagesPerDay: number;
        maxTokensPerMessage: number;
        canUploadImages: boolean;
    };
}

export const useUsageStats = () => {
    const [usageStats, setUsageStats] = useState<UsageStats | null>(null);
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const loadUsageStats = async () => {
        try {
            setIsLoading(true);
            setError(null);
            const response = await fetch(`${API_BASE_URL}/usage/stats`, {
                credentials: 'include', // Incluir cookies HTTP-only
            });

            if (response.ok) {
                const payload = await response.json();
                const stats = payload?.data ?? payload;
                const rawStats = stats?.stats ?? stats;
                setUsageStats({
                    todayMessages: Number(rawStats.todayMessages ?? rawStats.total_requests ?? 0),
                    todayTokens: Number(rawStats.todayTokens ?? ((rawStats.tokens_input ?? 0) + (rawStats.tokens_output ?? 0))),
                    totalMessages: Number(rawStats.totalMessages ?? rawStats.total_requests ?? 0),
                    totalTokens: Number(rawStats.totalTokens ?? ((rawStats.tokens_input ?? 0) + (rawStats.tokens_output ?? 0))),
                    tier: String(rawStats.tier ?? 'registered'),
                    limits: {
                        messagesPerDay: Number(rawStats.limits?.messagesPerDay ?? 50),
                        maxTokensPerMessage: Number(rawStats.limits?.maxTokensPerMessage ?? 4096),
                        canUploadImages: Boolean(rawStats.limits?.canUploadImages ?? false),
                    },
                });
            } else if (response.status === 429) {
                let errorData: { error?: string; limit?: number; reset_at?: number; retry_after?: string } = {};
                try {
                    errorData = await response.json();
                } catch { /* ignore parse error */ }
                setUsageStats({
                    todayMessages: 0,
                    todayTokens: 0,
                    totalMessages: 0,
                    totalTokens: 0,
                    tier: 'registered',
                    limits: {
                        messagesPerDay: Number(errorData.limit ?? 10),
                        maxTokensPerMessage: 4096,
                        canUploadImages: false,
                    },
                });
                setError(`Límite de mensajes alcanzado (${errorData.limit ?? '?'} mensajes/mes). Intenta de nuevo en ${errorData.retry_after ?? '?'}s.`);
            } else {
                console.warn('Error cargando estadísticas de uso:', response.status);
                setUsageStats(null);
                setError('Error cargando estadísticas de uso');
            }
        } catch (err) {
            console.warn('Error cargando estadísticas de uso:', err);
            setUsageStats(null);
            setError('Error cargando estadísticas de uso');
        } finally {
            setIsLoading(false);
        }
    };

    useEffect(() => {
        loadUsageStats();
    }, []);

    return {
        usageStats,
        isLoading,
        error,
        loadUsageStats
    };
};
