import { useSources } from '../../../hooks/useSources';

export const useWorkspace = () => {
  const { sources, selectedSource, selectSource, loading } = useSources();

  return {
    sources,
    selectedSource,
    selectSource,
    loading,
  };
};
