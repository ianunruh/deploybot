import { useEffect, useRef } from "react";

type FetcherState = "idle" | "loading" | "submitting";

export function useFetcherResult<T>(
  fetcher: { state: FetcherState; data?: T },
  onResult: (data: T) => void,
): void {
  const wasBusy = useRef(false);
  const onResultRef = useRef(onResult);

  useEffect(() => {
    onResultRef.current = onResult;
  }, [onResult]);

  useEffect(() => {
    if (fetcher.state !== "idle") {
      wasBusy.current = true;
      return;
    }
    if (!wasBusy.current || fetcher.data == null) return;
    wasBusy.current = false;
    onResultRef.current(fetcher.data);
  }, [fetcher.state, fetcher.data]);
}
