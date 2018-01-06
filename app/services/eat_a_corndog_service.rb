# frozen_string_literal: true

class EatACorndogService < BaseService
  include RoutingHelper

  def call(account, in_reply_to)
    text = in_reply_to.text

    if in_reply_to.local?
      html = Nokogiri::HTML(text)
      html.xpath('//a').each do |node|
        mentioned = Account.find_by(url: node['href'])

        if mentioned.present?
          node.replace(Nokogiri::XML::Text.new(mentioned.acct, html))
        end
      end

      text = html.text
    end

    words = text.downcase.split.reject { |word| word.include? 'tenshi' }

    case words[0]
    when 'eat'
      Rails.root.join('tenshi_eating_a_corndog', 'original.png').open do |file|
        post account, in_reply_to, file: file, description: 'Tenshi eating a corndog'
      end

    when 'let'
      case words[2]
      when 'eat'
        eater = in_reply_to.account
        unless words[1] == 'me'
          words[1] =~ Account::MENTION_RE
          eater = AccountSearchService.new.call($1.nil? ? words[1] : $1, 1, true)[0]
        end

        if eater.avatar?
          Tempfile.create 'mastodon-tenshi-avatar' do |avatar|
            eater.avatar.copy_to_local_file :original, avatar.path
            composite_and_post account, in_reply_to, eater, avatar.path
          end
        else
          composite_and_post account, in_reply_to, eater, Rails.public_path.to_s + eater.avatar.url
        end
      end
    end
  end

  private

  def composite_and_post(account, in_reply_to, eater, avatar)
    Tempfile.create ['mastodon-tenshi-eating-a-corndog', '.png'] do |composited|
      background = Rails.root.join('tenshi_eating_a_corndog', 'background.png').to_s
      mask = Rails.root.join('tenshi_eating_a_corndog', 'mask.png').to_s

      Process.wait spawn(
        'convert', background, '(', avatar, '-resize',
        '244x244', '-geometry', '+208+171', ')',
        '-composite', mask, '-composite', composited.path
      )

      if $?.success?
        post account, in_reply_to, file: composited, description: "@#{eater.acct} eating a corndog"
      else
        Rails.logger.error $?
      end
    end
  end

  def post(account, in_reply_to, attributes)
    media = account.media_attachments.create!(attributes)
    media_url = medium_url(media)

    PostStatusService.new.call(
      account, "@#{in_reply_to.account.acct} #{media_url}",
      in_reply_to, media_ids: [media.id]
    )
  end
end
