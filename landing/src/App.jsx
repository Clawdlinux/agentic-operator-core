import './index.css';
import { useTheme } from './hooks/useTheme';

import Navigation from './components/Navigation';
import Hero from './components/Hero';
import StatsBar from './components/StatsBar';
import Comparison from './components/Comparison';
import Offerings from './components/Offerings';
import Features from './components/Features';
import UseCases from './components/UseCases';
import Products from './components/Products';
import Architecture from './components/Architecture';
import Trust from './components/Trust';
import Quickstart from './components/Quickstart';
import OpenSource from './components/OpenSource';
import Waitlist from './components/Waitlist';
import Footer from './components/Footer';

export default function App() {
  const { currentTheme } = useTheme();
  
  return (
    <div
      style={{
        background: currentTheme.bg.primary,
        minHeight: '100vh',
        cursor: 'default',
        scrollBehavior: 'smooth',
        transition: 'background-color 300ms ease-in-out',
      }}
    >
      <Navigation />

      <main>
        <section id="home">
          <Hero />
        </section>

        <StatsBar />

        <Comparison />

        <Quickstart />

        <section id="features">
          <Offerings />
          <Features />
        </section>

        <section id="use-cases">
          <UseCases />
        </section>

        <Products />

        <section id="architecture">
          <Architecture />
        </section>

        <Trust />

        <OpenSource />

        <Waitlist />
      </main>

      <Footer />
    </div>
  );
}
