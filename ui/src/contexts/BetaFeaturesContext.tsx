import React, { createContext, useContext, useState, useEffect, ReactNode } from 'react';

interface BetaFeaturesContextType {
  isBetaEnabled: boolean;
}

const BetaFeaturesContext = createContext<BetaFeaturesContextType>({ isBetaEnabled: false });

/**
 * Provider that checks for ?beta=true query parameter on initial load.
 * Once enabled, beta features remain enabled for the session.
 */
export function BetaFeaturesProvider({ children }: { children: ReactNode }) {
  const [isBetaEnabled, setIsBetaEnabled] = useState(() => {
    // Check URL on initial load
    const params = new URLSearchParams(window.location.search);
    return params.get('beta') === 'true';
  });

  useEffect(() => {
    // Also check on URL changes (e.g., if user adds ?beta=true later)
    const checkBetaParam = () => {
      const params = new URLSearchParams(window.location.search);
      if (params.get('beta') === 'true') {
        setIsBetaEnabled(true);
      }
    };

    window.addEventListener('popstate', checkBetaParam);
    return () => window.removeEventListener('popstate', checkBetaParam);
  }, []);

  return (
    <BetaFeaturesContext.Provider value={{ isBetaEnabled }}>
      {children}
    </BetaFeaturesContext.Provider>
  );
}

/**
 * Hook to check if beta features are enabled.
 */
export function useBetaFeatures(): boolean {
  const context = useContext(BetaFeaturesContext);
  return context.isBetaEnabled;
}

export default BetaFeaturesContext;
