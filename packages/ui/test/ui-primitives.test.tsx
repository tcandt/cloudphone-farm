import React from 'react';
import { render, screen, fireEvent, act } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { Button, Modal, ConfirmDialog, ToastProvider, useToastStore, ErrorState, EmptyState } from '../src';

describe('UI Primitives', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('renders Button variants correctly', () => {
    render(<Button variant="primary">PrimaryBtn</Button>);
    expect(screen.getByText('PrimaryBtn')).toBeInTheDocument();
  });

  it('handles Button click events', () => {
    const onClick = vi.fn();
    render(<Button onClick={onClick}>Clickable</Button>);
    fireEvent.click(screen.getByText('Clickable'));
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it('renders Modal correctly when open', () => {
    render(
      <Modal isOpen={true} onClose={() => {}} title="Test Modal">
        ModalContent
      </Modal>
    );
    expect(screen.getByText('Test Modal')).toBeInTheDocument();
    expect(screen.getByText('ModalContent')).toBeInTheDocument();
  });

  it('closes Modal on Escape key', () => {
    const onClose = vi.fn();
    render(
      <Modal isOpen={true} onClose={onClose} title="Test Modal">
        ModalContent
      </Modal>
    );
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('handles ConfirmDialog confirm action', () => {
    const onConfirm = vi.fn();
    render(
      <ConfirmDialog isOpen={true} onClose={() => {}} onConfirm={onConfirm} title="Confirm action" />
    );
    fireEvent.click(screen.getByText('Confirm'));
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it('handles ConfirmDialog cancel action', () => {
    const onClose = vi.fn();
    render(
      <ConfirmDialog isOpen={true} onClose={onClose} onConfirm={() => {}} title="Confirm action" />
    );
    fireEvent.click(screen.getByText('Cancel'));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('adds and auto-removes Toast', () => {
    render(<ToastProvider />);
    const addToast = useToastStore.getState().addToast;
    
    act(() => {
      addToast({ type: 'success', title: 'Test Toast' });
    });
    expect(screen.getByText('Test Toast')).toBeInTheDocument();

    // Fast-forward 5s
    act(() => {
      vi.advanceTimersByTime(5000);
    });
    expect(screen.queryByText('Test Toast')).not.toBeInTheDocument();
  });

  it('manually removes Toast', () => {
    render(<ToastProvider />);
    const addToast = useToastStore.getState().addToast;
    
    act(() => {
      addToast({ type: 'error', title: 'Manual Toast' });
    });
    expect(screen.getByText('Manual Toast')).toBeInTheDocument();

    const closeBtn = screen.getByLabelText('Close notification');
    fireEvent.click(closeBtn);
    
    expect(screen.queryByText('Manual Toast')).not.toBeInTheDocument();
  });

  it('renders EmptyState correctly', () => {
    render(<EmptyState title="No Items" description="Try later" />);
    expect(screen.getByText('No Items')).toBeInTheDocument();
    expect(screen.getByText('Try later')).toBeInTheDocument();
  });

  it('renders ErrorState and triggers retry', () => {
    const onRetry = vi.fn();
    render(<ErrorState title="Fatal Error" onRetry={onRetry} />);
    expect(screen.getByText('Fatal Error')).toBeInTheDocument();
    const retryBtn = screen.getByText('Try Again');
    fireEvent.click(retryBtn);
    expect(onRetry).toHaveBeenCalledTimes(1);
  });
});
