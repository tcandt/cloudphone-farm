import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { Button, Modal, ErrorState, EmptyState } from '../src';

describe('UI Primitives', () => {
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

  it('does not render Modal when closed', () => {
    render(
      <Modal isOpen={false} onClose={() => {}} title="Test Modal">
        ModalContent
      </Modal>
    );
    expect(screen.queryByText('Test Modal')).not.toBeInTheDocument();
    expect(screen.queryByText('ModalContent')).not.toBeInTheDocument();
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
