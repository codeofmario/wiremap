import { Component } from 'react';
import { ErrorBoundaryProps, ErrorBoundaryState } from './ErrorBoundary.vm';

export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { hasError: false };

  static getDerivedStateFromError(): ErrorBoundaryState {
    return { hasError: true };
  }

  componentDidCatch() {
    // Reset after a tick so the component can re-render with fresh data
    setTimeout(() => this.setState({ hasError: false }), 100);
  }

  render() {
    if (this.state.hasError) {
      return this.props.fallback ?? null;
    }
    return this.props.children;
  }
}
