import { Component, type ErrorInfo, type ReactNode } from "react";

interface Props {
  children: ReactNode;
  pageName?: string;
  onError?: (error: Error, errorInfo: ErrorInfo) => void;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false, error: null };

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error(
      `[ErrorBoundary] React error in ${this.props.pageName || "unknown"}:`,
      error,
      errorInfo
    );
    this.props.onError?.(error, errorInfo);
    window.dispatchEvent(
      new CustomEvent("bb.react-notification", {
        detail: {
          module: "bytebase",
          style: "CRITICAL",
          title: `Page Error: ${this.props.pageName || "Unknown"}`,
          description: error.message,
        },
      })
    );
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="flex flex-col items-center justify-center h-full gap-4 p-8">
          <div className="text-red-500 text-lg font-medium">
            Something went wrong
          </div>
          <div className="text-gray-500 text-sm max-w-md text-center">
            {this.state.error?.message}
          </div>
          <button
            className="px-4 py-2 bg-accent text-white rounded hover:bg-accent-hover"
            onClick={() => this.setState({ hasError: false, error: null })}
          >
            Try Again
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}
