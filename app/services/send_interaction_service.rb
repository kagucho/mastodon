# frozen_string_literal: true

class SendInteractionService < BaseService
  # Send an Atom representation of an interaction to a remote Salmon endpoint
  # @param [String] Entry XML
  # @param [Account] source_account
  # @param [Account] target_account
  def call(xml, source_account, target_account)
    @xml            = xml
    @source_account = source_account
    @target_account = target_account

    return if block_notification?

    envelope = OStatus2::Salmon::MagicEnvelope.new(@xml, @source_account.keypair)
    OStatus2::Salmon::MagicEnvelope.post_xml(@target_account.salmon_url, envelope.to_xml)
  end

  private

  def block_notification?
    DomainBlock.blocked?(@target_account.domain)
  end
end
