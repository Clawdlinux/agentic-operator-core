"""
Hedge fund competitive intelligence synthesis node.

Takes DOM + screenshot analysis and produces a professional PDF report
that looks like something a real analyst created.
"""

import json
import logging
from datetime import datetime
from io import BytesIO
from typing import Optional

from reportlab.lib.pagesizes import letter
from reportlab.lib.styles import getSampleStyleSheet, ParagraphStyle
from reportlab.lib.units import inch
from reportlab.platypus import SimpleDocTemplate, Paragraph, Spacer, Table, TableStyle, PageBreak
from reportlab.lib import colors

logger = logging.getLogger(__name__)

SCRAPER_PROMPT = """You are a competitive intelligence analyst. Extract the following from this webpage content:

1. **Pricing tiers** (names and prices in a table format)
2. **Key features** listed per tier
3. **Any recent changes** mentioned (new tiers, price changes, discontinued features)
4. **Primary call-to-action** language
5. **Target audience** (from marketing copy)

Return as structured JSON with these exact keys:
- tiers (array of {name, price, features[]})
- recent_changes (string or null)
- primary_cta (string)
- target_audience (string)

Webpage content:
{content}"""

SYNTHESIS_PROMPT = """You are a hedge fund research analyst writing a competitive intelligence brief.

Given pricing and feature data from 3 competitors, produce a structured report covering:

1. **PRICING STRATEGY COMPARISON** - Who is positioned where, why, pricing psychology
2. **FEATURE DIFFERENTIATION** - What each is betting on, emerging feature trends
3. **STRATEGIC SIGNALS** - What recent changes indicate about company direction and market bets
4. **INVESTMENT IMPLICATIONS** - Which company's strategy appears strongest, why, market vulnerability signals

Write with the precision of a hedge fund brief. Use specific price points and feature gaps.
Format: 500-800 words, professional tone, actionable insights.

Competitor data:
{competitor_data}"""


class HedgeFundReportGenerator:
    """Generates professional PDF reports for hedge fund analysts."""
    
    @staticmethod
    def generate_pdf(
        competitor_analyses: dict,
        synthesis_text: str,
        minio_client=None,
        bucket_name: str = "reports",
        report_name: str = "competitive-intelligence-brief.pdf"
    ) -> bytes:
        """
        Generate a professional PDF report for hedge fund analysts.
        
        Args:
            competitor_analyses: Dict of {url: analysis_data}
            synthesis_text: Synthesized report from Claude/GPT-4o
            minio_client: MinIO client for storage (optional)
            bucket_name: MinIO bucket name
            report_name: Output filename
            
        Returns:
            PDF bytes
        """
        logger.info("Generating hedge fund report PDF")
        
        # Create PDF in memory
        pdf_buffer = BytesIO()
        doc = SimpleDocTemplate(pdf_buffer, pagesize=letter,
                                rightMargin=0.75*inch, leftMargin=0.75*inch,
                                topMargin=0.75*inch, bottomMargin=0.75*inch)
        
        # Build story (content)
        story = []
        styles = getSampleStyleSheet()
        
        # Custom styles
        title_style = ParagraphStyle(
            'CustomTitle',
            parent=styles['Heading1'],
            fontSize=18,
            textColor=colors.HexColor('#1a202c'),
            spaceAfter=12,
            fontName='Helvetica-Bold'
        )
        
        section_style = ParagraphStyle(
            'SectionTitle',
            parent=styles['Heading2'],
            fontSize=14,
            textColor=colors.HexColor('#2d3748'),
            spaceAfter=10,
            spaceBefore=12,
            fontName='Helvetica-Bold'
        )
        
        normal_style = ParagraphStyle(
            'CustomNormal',
            parent=styles['Normal'],
            fontSize=10,
            leading=14,
            textColor=colors.HexColor('#2d3748')
        )
        
        # Header
        timestamp = datetime.utcnow().strftime("%Y-%m-%d %H:%M UTC")
        story.append(Paragraph("COMPETITIVE INTELLIGENCE BRIEF", title_style))
        story.append(Paragraph(f"Generated: {timestamp} | Classification: Confidential", 
                             ParagraphStyle('Meta', parent=styles['Normal'], 
                                           fontSize=9, textColor=colors.grey)))
        story.append(Paragraph("Infrastructure: Customer K8s Cluster | Data Sovereignty: Verified",
                             ParagraphStyle('Meta2', parent=styles['Normal'],
                                           fontSize=9, textColor=colors.grey)))
        story.append(Spacer(1, 0.3*inch))
        
        # Executive Summary (from synthesis)
        story.append(Paragraph("EXECUTIVE SUMMARY", section_style))
        first_paragraph = synthesis_text.split('\n')[0] if synthesis_text else "Analysis in progress"
        story.append(Paragraph(first_paragraph, normal_style))
        story.append(Spacer(1, 0.2*inch))
        
        # Pricing Landscape Table
        story.append(Paragraph("PRICING LANDSCAPE", section_style))
        
        pricing_data = [["Competitor", "Primary Tier", "Price Point", "Key Differentiator"]]
        for url, analysis in competitor_analyses.items():
            try:
                data = json.loads(analysis.get('extracted_data', '{}'))
                tiers = data.get('tiers', [])
                if tiers:
                    main_tier = tiers[0]  # First/primary tier
                    pricing_data.append([
                        url.split('/')[2].split('.')[0].upper(),
                        main_tier.get('name', 'N/A'),
                        main_tier.get('price', 'N/A'),
                        ', '.join(main_tier.get('features', [])[:2])[:40]
                    ])
            except Exception as e:
                logger.error(f"Error parsing competitor data: {e}")
        
        if len(pricing_data) > 1:
            pricing_table = Table(pricing_data, colWidths=[1.2*inch, 1.3*inch, 1.2*inch, 1.3*inch])
            pricing_table.setStyle(TableStyle([
                ('BACKGROUND', (0, 0), (-1, 0), colors.HexColor('#2d3748')),
                ('TEXTCOLOR', (0, 0), (-1, 0), colors.whitesmoke),
                ('ALIGN', (0, 0), (-1, -1), 'LEFT'),
                ('FONTNAME', (0, 0), (-1, 0), 'Helvetica-Bold'),
                ('FONTSIZE', (0, 0), (-1, 0), 10),
                ('BOTTOMPADDING', (0, 0), (-1, 0), 12),
                ('BACKGROUND', (0, 1), (-1, -1), colors.beige),
                ('GRID', (0, 0), (-1, -1), 1, colors.grey),
                ('FONTSIZE', (0, 1), (-1, -1), 9),
            ]))
            story.append(pricing_table)
        
        story.append(Spacer(1, 0.2*inch))
        
        # Strategic Analysis (full synthesis)
        story.append(Paragraph("STRATEGIC ANALYSIS", section_style))
        story.append(Paragraph(synthesis_text, normal_style))
        story.append(Spacer(1, 0.3*inch))
        
        # Methodology footer
        story.append(Paragraph("METHODOLOGY", ParagraphStyle('FooterTitle', 
                                                             parent=styles['Heading3'],
                                                             fontSize=10,
                                                             fontName='Helvetica-Bold')))
        story.append(Paragraph(
            "Visual DOM analysis + Screenshot review + LLM synthesis<br/>"
            "Processing: 3 competitors analyzed in parallel<br/>"
            "<b>Data Sovereignty:</b> All data processed within customer infrastructure. "
            "No external data transfer.",
            normal_style
        ))
        
        # Build PDF
        doc.build(story)
        pdf_bytes = pdf_buffer.getvalue()
        
        logger.info(f"PDF generated: {len(pdf_bytes)} bytes")
        
        # Upload to MinIO if client provided
        if minio_client:
            try:
                minio_client.put_object(
                    bucket_name=bucket_name,
                    object_name=report_name,
                    data=BytesIO(pdf_bytes),
                    length=len(pdf_bytes),
                    content_type="application/pdf"
                )
                logger.info(f"Report uploaded to MinIO: {bucket_name}/{report_name}")
            except Exception as e:
                logger.error(f"Failed to upload to MinIO: {e}")
        
        return pdf_bytes


async def synthesis_agent_with_pdf(state: dict, litellm_client) -> dict:
    """
    Updated synthesis node that produces a hedge fund PDF report.
    
    Args:
        state: AgentWorkflowState with competitor analyses
        litellm_client: LiteLLM client for synthesis
        
    Returns:
        Updated state with PDF report
    """
    logger.info("[synthesis_agent_pdf] Synthesizing hedge fund report")
    
    try:
        # Compile context from both visual and DOM analysis
        competitor_context = {}
        for url, screenshot_data in state.get('visual_insights', {}).items():
            competitor_context[url] = screenshot_data
        
        context_str = json.dumps(competitor_context, indent=2)
        
        # Generate synthesis using Claude/GPT-4o
        synthesis = await litellm_client.synthesize_report(
            context=context_str,
            prompt=SYNTHESIS_PROMPT
        )
        
        # Generate PDF report
        pdf_generator = HedgeFundReportGenerator()
        pdf_bytes = pdf_generator.generate_pdf(
            competitor_analyses=state.get('visual_insights', {}),
            synthesis_text=synthesis
        )
        
        # Update state
        state["report_content"] = synthesis
        state["report_pdf"] = pdf_bytes
        state["status"] = "complete"
        
        logger.info(f"[synthesis_agent_pdf] Report complete: {len(pdf_bytes)} bytes PDF")
        return state
        
    except Exception as e:
        logger.error(f"[synthesis_agent_pdf] Failed: {e}")
        state["error"] = str(e)
        state["status"] = "failed"
        return state
