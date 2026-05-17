import React, { ErrorInfo, ReactNode } from "react";

interface Props {
  pageName: string;
  children: ReactNode;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

export class ReactErrorBoundary extends React.Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error(`[ErrorBoundary:${this.props.pageName}] Render error:`, error, errorInfo.componentStack);
  }

  handleRetry = () => {
    this.setState({ hasError: false, error: null });
  };

  render() {
    if (this.state.hasError) {
      return (
        <div className="flex flex-col items-center justify-center min-h-[400px] p-8 text-center bg-gray-50 dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-800">
          <h2 className="text-xl font-semibold text-red-600 mb-2">Something went wrong in {this.props.pageName}</h2>
          <p className="text-gray-600 dark:text-gray-400 mb-6 max-w-md">
            The application encountered an unexpected error while rendering this component.
          </p>
          <button 
            onClick={this.handleRetry}
            className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 transition-colors"
          >
            Retry
          </button>
          
          {import.meta.env.DEV && this.state.error && (
            <div className="mt-8 p-4 bg-gray-100 dark:bg-gray-800 rounded w-full overflow-auto text-left">
              <pre className="text-xs text-red-500 whitespace-pre-wrap">
                {this.state.error.stack || this.state.error.message}
              </pre>
            </div>
          )}
        </div>
      );
    }

    return this.props.children;
  }
}
