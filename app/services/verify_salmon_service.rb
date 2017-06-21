# frozen_string_literal: true

class VerifySalmonService < BaseService
  include AuthorExtractor

  def call(payload)
    envelope = OStatus2::Salmon::MagicEnvelope.new(payload)

    xml = Nokogiri::XML(envelope.body)
    xml.encoding = 'utf-8'

    account = author_from_xml(xml.at_xpath('/xmlns:entry', xmlns: TagManager::XMLNS))

    if account.nil?
      false
    else
      envelope.verify(account.keypair)
    end
  end
end
